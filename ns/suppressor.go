package ns

import (
	"math"

	"sonora/fft"
)

type Suppressor struct {
	config         Config
	params         suppressionParams
	fftProcessor   *fft.FFT4G
	noiseEst       *noiseEstimator
	wiener         *wienerFilter
	speechProbEst  *speechProbabilityEstimator
	signalModelEst *signalModelEstimator

	analysisBuffer [fftSize]float32
	synthBuffer    [fftSize]float32
	overlapBuf     [overlapSize]float32
	window         [fftSize]float32

	re [numFreqBins]float32
	im [numFreqBins]float32

	speechProb  [numFreqBins]float32
	signalSpec  [numFreqBins]float32
}

func NewSuppressor(cfg Config) *Suppressor {
	s := &Suppressor{
		config:         cfg,
		params:         getSuppressionParams(cfg.Level),
		fftProcessor:   fft.NewFFT4G(fftSize),
		noiseEst:       newNoiseEstimator(),
		wiener:         newWienerFilter(),
		speechProbEst:  newSpeechProbabilityEstimator(),
		signalModelEst: newSignalModelEstimator(),
	}
	s.initWindow()
	return s
}

func (s *Suppressor) initWindow() {
	for i := 0; i < fftSize; i++ {
		t := float32(i) / float32(fftSize)
		s.window[i] = 0.5 - 0.5*cosf(2*pi*t)
	}
}

func (s *Suppressor) Process(frame []float32) {
	if len(frame) < frameLength {
		return
	}

	copy(s.analysisBuffer[:overlapSize], s.overlapBuf[:])
	copy(s.analysisBuffer[overlapSize:], frame[:frameLength])
	copy(s.overlapBuf[:], frame[frameLength-overlapSize:frameLength])

	for i := 0; i < fftSize; i++ {
		s.analysisBuffer[i] *= s.window[i]
	}

	fftData := make([]float32, fftSize)
	copy(fftData, s.analysisBuffer[:])
	s.fftProcessor.Forward(fftData)

	s.re[0] = fftData[0]
	s.im[0] = 0
	s.re[numFreqBins-1] = fftData[1]
	s.im[numFreqBins-1] = 0
	for i := 1; i < numFreqBins-1; i++ {
		s.re[i] = fftData[2*i]
		s.im[i] = fftData[2*i+1]
	}

	for i := 0; i < numFreqBins; i++ {
		power := s.re[i]*s.re[i] + s.im[i]*s.im[i]
		if power < 1e-10 {
			power = 1e-10
		}
		s.signalSpec[i] = fastLog(power)
	}

	s.noiseEst.Update(s.signalSpec)
	noiseSpec := s.noiseEst.Spectrum()

	model := &signalModel{}
	s.signalModelEst.Update(s.signalSpec, noiseSpec, model)
	s.speechProbEst.Estimate(model, noiseSpec, s.signalSpec, s.speechProb[:])
	s.wiener.Update(s.signalSpec, noiseSpec, s.speechProb, s.params)
	s.wiener.Apply(s.re[:], s.im[:])

	fftData[0] = s.re[0]
	fftData[1] = s.re[numFreqBins-1]
	for i := 1; i < numFreqBins-1; i++ {
		fftData[2*i] = s.re[i]
		fftData[2*i+1] = s.im[i]
	}

	s.fftProcessor.Inverse(fftData)

	for i := 0; i < fftSize; i++ {
		s.synthBuffer[i] = fftData[i] * s.window[i]
	}

	for i := 0; i < frameLength; i++ {
		if i < overlapSize {
			frame[i] = s.synthBuffer[i]
		} else {
			frame[i] = s.synthBuffer[i]
		}
	}
}

func (s *Suppressor) Reset() {
	clear(s.overlapBuf[:])
	clear(s.analysisBuffer[:])
	clear(s.synthBuffer[:])
}

const pi = math.Pi

func cosf(x float32) float32 {
	return float32(math.Cos(float64(x)))
}
