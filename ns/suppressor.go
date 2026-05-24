package ns

import (
	"math"

	"aeol/fft"
)

// Suppressor performs spectral subtraction noise suppression using overlap-add FFT processing.
// it operates on 160-sample FloatS16 frames (10ms at 16kHz) with a 256-point FFT and 96-sample overlap.
// instances are not safe for concurrent use.
type Suppressor struct {
	config         Config
	params         suppressionParams
	fftProcessor   fft.FFT
	noiseEst       *noiseEstimator
	wiener         *wienerFilter
	speechProbEst  *speechProbabilityEstimator
	signalModelEst *signalModelEstimator

	analysisBuffer [fftSize]float32
	synthBuffer    [fftSize]float32
	overlapBuf     [overlapSize]float32
	synthOverlap   [overlapSize]float32
	upperBandDelayBuf [2][overlapSize]float32
	window         [fftSize]float32

	re [numFreqBins]float32
	im [numFreqBins]float32

	fftBuf [fftSize]float32

	speechProb  [numFreqBins]float32
	signalSpec  [numFreqBins]float32
}

// NewSuppressor creates a new noise suppressor with the given config.
// cfg sets suppression aggressiveness; fftFactory is optional and selects the FFT backend (default: pure Go Ooura).
func NewSuppressor(cfg Config, fftFactory ...fft.Factory) *Suppressor {
	factory := fft.DefaultFactory
	if len(fftFactory) > 0 && fftFactory[0] != nil {
		factory = fftFactory[0]
	}
	s := &Suppressor{
		config:         cfg,
		params:         getSuppressionParams(cfg.Level),
		fftProcessor:   factory(fftSize),
		noiseEst:       newNoiseEstimator(),
		wiener:         newWienerFilter(),
		speechProbEst:  newSpeechProbabilityEstimator(),
		signalModelEst: newSignalModelEstimator(),
	}
	s.initWindow()
	return s
}

func (s *Suppressor) initWindow() {
	for i := 0; i < overlapSize; i++ {
		s.window[i] = float32(math.Sin(math.Pi * float64(i) / float64(2*overlapSize)))
	}
	for i := overlapSize; i <= frameLength; i++ {
		s.window[i] = 1.0
	}
	for i := 0; i < fftSize-frameLength-1; i++ {
		s.window[frameLength+1+i] = s.window[overlapSize-1-i]
	}
}

// Process performs one frame of noise suppression in-place.
// frame must contain exactly 160 FloatS16 samples (float32 in [-32768, 32767]).
// applies analysis windowing, forward FFT, Wiener filtering, inverse FFT, and overlap-add synthesis.
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

	copy(s.fftBuf[:], s.analysisBuffer[:])
	s.fftProcessor.Forward(s.fftBuf[:])

	s.re[0] = s.fftBuf[0]
	s.im[0] = 0
	s.re[numFreqBins-1] = s.fftBuf[1]
	s.im[numFreqBins-1] = 0
	for i := 1; i < numFreqBins-1; i++ {
		s.re[i] = s.fftBuf[2*i]
		s.im[i] = s.fftBuf[2*i+1]
	}

	for i := 0; i < numFreqBins; i++ {
		s.signalSpec[i] = fastLog(s.re[i]*s.re[i] + s.im[i]*s.im[i])
	}

	s.noiseEst.Update(s.signalSpec)
	noiseSpec := s.noiseEst.Spectrum()

	model := &signalModel{}
	s.signalModelEst.Update(s.signalSpec, noiseSpec, model)
	s.speechProbEst.Estimate(model, noiseSpec, s.signalSpec, s.speechProb[:])
	s.wiener.Update(s.signalSpec, noiseSpec, s.speechProb, s.params)
	s.wiener.Apply(s.re[:], s.im[:])

	s.fftBuf[0] = s.re[0]
	s.fftBuf[1] = s.re[numFreqBins-1]
	for i := 1; i < numFreqBins-1; i++ {
		s.fftBuf[2*i] = s.re[i]
		s.fftBuf[2*i+1] = s.im[i]
	}

	s.fftProcessor.Inverse(s.fftBuf[:])

	for i := 0; i < fftSize; i++ {
		s.synthBuffer[i] = s.fftBuf[i] * s.window[i]
	}

	for i := 0; i < overlapSize; i++ {
		frame[i] = s.synthOverlap[i] + s.synthBuffer[i]
	}
	copy(frame[overlapSize:frameLength], s.synthBuffer[overlapSize:frameLength])
	copy(s.synthOverlap[:], s.synthBuffer[frameLength:fftSize])
}

// Reset clears all internal state including overlap buffers, noise estimator, and synthesis state.
// call this when reusing a Suppressor for a new audio stream.
func (s *Suppressor) Reset() {
	s.noiseEst = newNoiseEstimator()
	s.wiener = newWienerFilter()
	s.speechProbEst = newSpeechProbabilityEstimator()
	s.signalModelEst = newSignalModelEstimator()
	clear(s.overlapBuf[:])
	clear(s.synthOverlap[:])
	clear(s.analysisBuffer[:])
	clear(s.synthBuffer[:])
	clear(s.upperBandDelayBuf[0][:])
	clear(s.upperBandDelayBuf[1][:])
}

// ProcessUpperBand applies gain-matched suppression to an upper frequency band.
// band is 0-based and indexes the upper bands above 8kHz (e.g. 0 = first upper band at 8-16kHz for 32kHz input).
// must be called after Process for the same frame; derives gain from the lower band Wiener filter state and speech probability.
// frame must contain exactly 160 FloatS16 samples and is modified in-place with a latency-compensating delay.
func (s *Suppressor) ProcessUpperBand(frame []float32, band int) {
	if len(frame) < frameLength {
		return
	}

	var avgFilterGain float32
	var avgSpeechProb float32
	for i := numFreqBins - 32 - 1; i < numFreqBins - 1; i++ {
		avgFilterGain += s.wiener.gains[i]
		avgSpeechProb += s.speechProb[i]
	}
	avgFilterGain /= 32.0
	avgSpeechProb /= 32.0

	gain := float32(0.5) * (1.0 + float32(math.Tanh(float64(2.0*avgSpeechProb-1.0))))
	if avgSpeechProb >= 0.5 {
		gain = 0.25*gain + 0.75*avgFilterGain
	} else {
		gain = 0.5*gain + 0.5*avgFilterGain
	}

	gain *= s.wiener.overallScale

	minGain := float32(1.0) / s.params.minOverDrive
	if gain < minGain {
		gain = minGain
	}
	if gain > 1.0 {
		gain = 1.0
	}

	samplesFromFrame := frameLength - overlapSize
	var delayedFrame [frameLength]float32

	copy(delayedFrame[:overlapSize], s.upperBandDelayBuf[band][:overlapSize])
	copy(delayedFrame[overlapSize:], frame[:samplesFromFrame])
	copy(s.upperBandDelayBuf[band][:overlapSize], frame[samplesFromFrame:frameLength])

	for i := 0; i < frameLength; i++ {
		frame[i] = delayedFrame[i] * gain
	}
}
