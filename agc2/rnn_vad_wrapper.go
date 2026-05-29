package agc2

import (
	"github.com/ZertGraf/aeol/agc2/rnn_vad"
	"github.com/ZertGraf/aeol/dsp"
	"github.com/ZertGraf/aeol/fft"
)

// Ensure RNNVADWrapper implements VADAnalyzer interface.
var _ VADAnalyzer = (*RNNVADWrapper)(nil)

const (
	kVadResetPeriodMs   = 1500
	kFrameDurationMs    = 10
	kNumFramesPerSecond = 100
)

// MonoVad is the single channel VAD interface.
type MonoVad interface {
	SampleRateHz() int
	Reset()
	Analyze(frame []float32) float32
}

// MonoVadImpl is the concrete implementation of MonoVad using rnn_vad features.
type MonoVadImpl struct {
	featuresExtractor *rnn_vad.FeaturesExtractor
	rnnVad            *rnn_vad.RNNVad
}

// NewMonoVadImpl creates a new instance of MonoVadImpl.
// fftFactory is optional and selects the FFT backend for spectral feature extraction.
func NewMonoVadImpl(fftFactory ...fft.Factory) *MonoVadImpl {
	return &MonoVadImpl{
		featuresExtractor: rnn_vad.NewFeaturesExtractor(fftFactory...),
		rnnVad:            rnn_vad.NewRNNVad(),
	}
}

// SampleRateHz returns the sample rate (Hz) required for the input frames.
func (m *MonoVadImpl) SampleRateHz() int {
	return 24000
}

// Reset resets the internal state.
func (m *MonoVadImpl) Reset() {
	m.rnnVad.Reset()
}

// Analyze analyzes an audio frame and returns the speech probability.
func (m *MonoVadImpl) Analyze(frame []float32) float32 {
	if len(frame) != 240 {
		panic("invalid frame size")
	}
	var featureVector [rnn_vad.FeatureVectorSize]float32
	isSilence := m.featuresExtractor.CheckSilenceComputeFeatures(frame, featureVector[:])
	return m.rnnVad.ComputeVadProbability(featureVector[:], isSilence)
}

// RNNVADWrapper wraps a single-channel Voice Activity Detector (VAD).
// It takes care of resampling the input frames to match the sample rate
// of the wrapped VAD and periodically resets the VAD.
type RNNVADWrapper struct {
	vadResetPeriodFrames int
	frameSize            int
	timeToVadReset       int
	vad                  MonoVad
	resampledBuffer      []float32
	resampler            *dsp.PushResampler
}

// NewRNNVADWrapper creates a new RNNVADWrapper with the default reset period.
// fftFactory is optional and selects the FFT backend for spectral feature extraction.
func NewRNNVADWrapper(sampleRateHz int, fftFactory ...fft.Factory) *RNNVADWrapper {
	return NewRNNVADWrapperWithPeriod(kVadResetPeriodMs, sampleRateHz, fftFactory...)
}

// NewRNNVADWrapperWithPeriod creates a new RNNVADWrapper with a specified reset period.
// fftFactory is optional and selects the FFT backend for spectral feature extraction.
func NewRNNVADWrapperWithPeriod(vadResetPeriodMs int, sampleRateHz int, fftFactory ...fft.Factory) *RNNVADWrapper {
	return NewRNNVADWrapperWithCustomVad(vadResetPeriodMs, NewMonoVadImpl(fftFactory...), sampleRateHz)
}

// NewRNNVADWrapperWithCustomVad creates a new RNNVADWrapper with a custom MonoVad implementation.
func NewRNNVADWrapperWithCustomVad(vadResetPeriodMs int, vad MonoVad, sampleRateHz int) *RNNVADWrapper {
	vadResetPeriodFrames := vadResetPeriodMs / kFrameDurationMs
	if vadResetPeriodFrames <= 1 {
		panic("vad_reset_period_ms must be equal to or greater than the duration of two frames")
	}

	frameSize := sampleRateHz / kNumFramesPerSecond
	resampledBufferSize := vad.SampleRateHz() / kNumFramesPerSecond

	w := &RNNVADWrapper{
		vadResetPeriodFrames: vadResetPeriodFrames,
		frameSize:            frameSize,
		timeToVadReset:       vadResetPeriodFrames,
		vad:                  vad,
		resampledBuffer:      make([]float32, resampledBufferSize),
		resampler:            dsp.NewPushResampler(sampleRateHz, vad.SampleRateHz(), 1),
	}

	w.vad.Reset()
	return w
}

// Analyze analyzes the input frame and returns the speech probability.
// It implements the VADAnalyzer interface.
func (w *RNNVADWrapper) Analyze(frame []float32) float32 {
	// Periodically reset the VAD.
	w.timeToVadReset--
	if w.timeToVadReset <= 0 {
		w.vad.Reset()
		w.timeToVadReset = w.vadResetPeriodFrames
	}

	if len(frame) != w.frameSize {
		panic("invalid frame size")
	}

	// Resample the input frame to the required sample rate.
	src := [][]float32{frame}
	dst := [][]float32{w.resampledBuffer}
	w.resampler.Resample(src, dst)

	return w.vad.Analyze(w.resampledBuffer)
}

// Reset resets the VAD internal state.
func (w *RNNVADWrapper) Reset() {
	w.vad.Reset()
	w.timeToVadReset = w.vadResetPeriodFrames
}
