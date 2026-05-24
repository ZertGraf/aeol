package sonora

import (
	"fmt"
	"math"
	"sync"

	"sonora/aec3"
	agc "sonora/agc2"
	"sonora/hpf"
	"sonora/ns"
)

// AudioProcessing is the main entry point for the audio processing pipeline.
// it combines high-pass filtering, echo cancellation (AEC3), noise suppression (NS),
// and automatic gain control (AGC2) into a single, configurable processor.
// all methods are safe for concurrent use; an internal mutex serializes access.
type AudioProcessing struct {
	mu sync.Mutex

	captureConfig StreamConfig
	renderConfig  StreamConfig
	config        Config

	highPassFilters  []*hpf.Filter
	noiseSuppressors []*ns.Suppressor
	gainControllers  []*agc.GainController2
	echoCancellers   []*aec3.EchoCanceller3

	captureBuffer *AudioBuffer
	renderBuffer  *AudioBuffer

	streamDelayMs int
	stats         AudioProcessingStats

	statsDbfs  float64
	statsErle  float64
	statsDelay int
}

func newAudioProcessing(captureConfig, renderConfig StreamConfig, config Config) (*AudioProcessing, error) {
	ap := &AudioProcessing{
		captureConfig: captureConfig,
		renderConfig:  renderConfig,
		config:        config,
		captureBuffer: NewAudioBuffer(captureConfig),
		renderBuffer:  NewAudioBuffer(renderConfig),
	}

	numChannels := int(captureConfig.NumChannels)

	if config.HighPassFilterEnabled() {
		ap.highPassFilters = make([]*hpf.Filter, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.highPassFilters[ch] = hpf.New(captureConfig.SampleRateHz)
		}
	}

	if config.NoiseSuppressionEnabled() {
		nsCfg := ns.Config{Level: ns.SuppressionLevel(config.NoiseSuppression.Level)}
		ap.noiseSuppressors = make([]*ns.Suppressor, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.noiseSuppressors[ch] = ns.NewSuppressor(nsCfg)
		}
	}

	if config.GainController2Enabled() {
		gc2Config := agc.Config{
			Enabled: true,
			AdaptiveDigital: agc.AdaptiveDigitalConfig{
				Enabled:                  config.GainController2.AdaptiveDigital.Enabled,
				DryRun:                   config.GainController2.AdaptiveDigital.DryRun,
				HeadroomDb:               config.GainController2.AdaptiveDigital.HeadroomDb,
				MaxGainDb:                config.GainController2.AdaptiveDigital.MaxGainDb,
				InitialGainDb:            config.GainController2.AdaptiveDigital.InitialGainDb,
				MaxGainChangeDbPerSecond: config.GainController2.AdaptiveDigital.MaxGainChangeDbPerSecond,
				MaxOutputNoiseLevelDbfs:  config.GainController2.AdaptiveDigital.MaxOutputNoiseLevelDbfs,
			},
			FixedDigital: agc.FixedDigitalConfig{
				GainDb: config.GainController2.FixedDigital.GainDb,
			},
		}
		ap.gainControllers = make([]*agc.GainController2, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.gainControllers[ch] = agc.NewGainController2(gc2Config)
		}
	}

	if config.EchoCancellerEnabled() {
		ap.echoCancellers = make([]*aec3.EchoCanceller3, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.echoCancellers[ch] = aec3.NewEchoCanceller3(
				aec3.DefaultConfig(),
				captureConfig.SampleRateHz,
				1,
			)
		}
	}

	return ap, nil
}

// ProcessCaptureFloatS16 runs the capture pipeline on de-interleaved FloatS16 samples.
// data[ch] must contain exactly FrameSize() samples in the range [-32768, 32767].
// the slice is modified in place; the caller reads processed audio from the same slice after return.
func (ap *AudioProcessing) ProcessCaptureFloatS16(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.processCaptureFloatLocked(data)
}

// ProcessCaptureFloatNormalized runs the capture pipeline on de-interleaved normalized samples.
// data[ch] must contain exactly FrameSize() samples in the range [-1, 1].
// samples are scaled to FloatS16 internally and converted back before returning.
func (ap *AudioProcessing) ProcessCaptureFloatNormalized(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	for ch := range data {
		for i := range data[ch] {
			data[ch][i] *= 32768.0
		}
	}

	err := ap.processCaptureFloatLocked(data)

	for ch := range data {
		for i := range data[ch] {
			data[ch][i] /= 32768.0
		}
	}

	return err
}

func (ap *AudioProcessing) processCaptureFloatLocked(data [][]float32) error {
	if len(data) == 0 || len(data[0]) == 0 {
		return nil
	}

	expectedFrames := ap.captureConfig.FrameSize()
	if len(data[0]) != expectedFrames {
		return fmt.Errorf("expected %d samples per channel, got %d", expectedFrames, len(data[0]))
	}

	numChannels := int(ap.captureConfig.NumChannels)
	if len(data) < numChannels {
		numChannels = len(data)
	}

	if ap.config.PreAmplifierEnabled() {
		gain := ap.config.PreAmplifier.Gain
		if gain != 1.0 {
			for ch := 0; ch < numChannels; ch++ {
				for i := range data[ch] {
					data[ch][i] *= gain
				}
			}
		}
	}

	if ap.config.CaptureLevelAdjustment != nil && ap.config.CaptureLevelAdjustment.Enabled {
		if ap.config.CaptureLevelAdjustment.PreGainDb != 0 {
			gain := float32(math.Pow(10.0, float64(ap.config.CaptureLevelAdjustment.PreGainDb)/20.0))
			for ch := 0; ch < numChannels; ch++ {
				for i := range data[ch] {
					data[ch][i] *= gain
				}
			}
		}
	}

	for ch := 0; ch < numChannels; ch++ {
		chData := data[ch]
		if len(ap.highPassFilters) > ch && ap.highPassFilters[ch] != nil {
			ap.highPassFilters[ch].Process(chData)
		}
	}

	ap.captureBuffer.CopyFromFloat(data)

	bands := ap.captureBuffer.Bands()
	if bands > 1 {
		ap.captureBuffer.SplitIntoFrequencyBands()
	}

	for ch := 0; ch < numChannels; ch++ {
		lowerBand := ap.captureBuffer.SplitChannel(ch, 0)

		if len(ap.echoCancellers) > ch && ap.echoCancellers[ch] != nil {
			for start := 0; start+aec3.BlockSize <= len(lowerBand); start += aec3.BlockSize {
				block := lowerBand[start : start+aec3.BlockSize]
				ap.echoCancellers[ch].ProcessCapture(block)
			}
		}

		if len(ap.noiseSuppressors) > ch && ap.noiseSuppressors[ch] != nil {
			ap.noiseSuppressors[ch].Process(lowerBand)
			if bands > 1 {
				ap.noiseSuppressors[ch].ProcessUpperBand(ap.captureBuffer.SplitChannel(ch, 1), 0)
				if bands > 2 {
					ap.noiseSuppressors[ch].ProcessUpperBand(ap.captureBuffer.SplitChannel(ch, 2), 1)
				}
			}
		}
	}

	if bands > 1 {
		ap.captureBuffer.MergeFrequencyBands()
	}

	ap.captureBuffer.CopyToFloat(data)

	for ch := 0; ch < numChannels; ch++ {
		chData := data[ch]
		if len(ap.gainControllers) > ch && ap.gainControllers[ch] != nil {
			ap.gainControllers[ch].Process(chData)
		}
	}

	if ap.config.CaptureLevelAdjustment != nil && ap.config.CaptureLevelAdjustment.Enabled {
		if ap.config.CaptureLevelAdjustment.PostGainDb != 0 {
			gain := float32(math.Pow(10.0, float64(ap.config.CaptureLevelAdjustment.PostGainDb)/20.0))
			for ch := 0; ch < numChannels; ch++ {
				for i := range data[ch] {
					data[ch][i] *= gain
				}
			}
		}
	}

	ap.updateStats(data)
	return nil
}

// ProcessRenderFloatS16 feeds far-end (speaker) audio to the echo canceller in FloatS16 format.
// data[ch] must contain exactly FrameSize() samples in the range [-32768, 32767].
// must be called before the corresponding ProcessCaptureFloatS16 call when AEC is enabled.
func (ap *AudioProcessing) ProcessRenderFloatS16(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.processRenderFloatLocked(data)
}

// ProcessRenderFloatNormalized feeds far-end (speaker) audio to the echo canceller in normalized format.
// data[ch] must contain exactly FrameSize() samples in the range [-1, 1].
// must be called before the corresponding ProcessCaptureFloatNormalized call when AEC is enabled.
func (ap *AudioProcessing) ProcessRenderFloatNormalized(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	for ch := range data {
		for i := range data[ch] {
			data[ch][i] *= 32768.0
		}
	}

	err := ap.processRenderFloatLocked(data)

	for ch := range data {
		for i := range data[ch] {
			data[ch][i] /= 32768.0
		}
	}

	return err
}

// processRenderFloatLocked feeds render (far-end) audio to echo cancellers.
// only the first render channel is used — stereo render is not supported.
func (ap *AudioProcessing) processRenderFloatLocked(data [][]float32) error {
	if len(data) == 0 || len(data[0]) == 0 {
		return nil
	}

	ap.renderBuffer.CopyFromFloat(data)

	if len(ap.echoCancellers) > 0 {
		bands := ap.renderBuffer.Bands()
		if bands > 1 {
			ap.renderBuffer.SplitIntoFrequencyBands()
		}

		lowerBand := ap.renderBuffer.SplitChannel(0, 0)

		for start := 0; start+aec3.BlockSize <= len(lowerBand); start += aec3.BlockSize {
			block := lowerBand[start : start+aec3.BlockSize]
			for _, ec := range ap.echoCancellers {
				if ec != nil {
					ec.ProcessRender(block)
				}
			}
		}
	}

	return nil
}

// ProcessCaptureInt16 runs the capture pipeline on an interleaved int16 frame.
// interleaved must contain exactly TotalSamples() elements in channel-interleaved order.
// samples are converted to FloatS16 internally and written back to the same slice.
func (ap *AudioProcessing) ProcessCaptureInt16(interleaved []int16) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	expectedSamples := ap.captureConfig.TotalSamples()
	if len(interleaved) != expectedSamples {
		return fmt.Errorf("expected %d samples, got %d", expectedSamples, len(interleaved))
	}

	ap.captureBuffer.CopyFromInterleaved(interleaved)

	channels := make([][]float32, ap.captureConfig.NumChannels)
	for ch := range channels {
		channels[ch] = ap.captureBuffer.Channel(ch)
	}

	if err := ap.processCaptureFloatLocked(channels); err != nil {
		return err
	}

	ap.captureBuffer.CopyToInterleaved(interleaved)
	return nil
}

// ProcessRenderInt16 feeds far-end (speaker) audio to the echo canceller as interleaved int16.
// interleaved must contain exactly TotalSamples() elements using the render stream layout.
// must be called before the corresponding ProcessCaptureInt16 call when AEC is enabled.
func (ap *AudioProcessing) ProcessRenderInt16(interleaved []int16) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	expectedSamples := ap.renderConfig.TotalSamples()
	if len(interleaved) != expectedSamples {
		return fmt.Errorf("expected %d samples, got %d", expectedSamples, len(interleaved))
	}

	ap.renderBuffer.CopyFromInterleaved(interleaved)

	channels := make([][]float32, ap.renderConfig.NumChannels)
	for ch := range channels {
		channels[ch] = ap.renderBuffer.Channel(ch)
	}

	return ap.processRenderFloatLocked(channels)
}

// ApplyConfig replaces the active processing configuration at runtime.
// stages that are newly enabled are initialized fresh; stages that are disabled are torn down.
// does not reset the internal state of stages that remain enabled across calls.
func (ap *AudioProcessing) ApplyConfig(config Config) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	numChannels := int(ap.captureConfig.NumChannels)

	if config.HighPassFilterEnabled() && len(ap.highPassFilters) == 0 {
		ap.highPassFilters = make([]*hpf.Filter, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.highPassFilters[ch] = hpf.New(ap.captureConfig.SampleRateHz)
		}
	} else if !config.HighPassFilterEnabled() {
		ap.highPassFilters = nil
	}

	if config.NoiseSuppressionEnabled() && len(ap.noiseSuppressors) == 0 {
		nsCfg := ns.Config{Level: ns.SuppressionLevel(config.NoiseSuppression.Level)}
		ap.noiseSuppressors = make([]*ns.Suppressor, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.noiseSuppressors[ch] = ns.NewSuppressor(nsCfg)
		}
	} else if !config.NoiseSuppressionEnabled() {
		ap.noiseSuppressors = nil
	}

	if config.GainController2Enabled() && len(ap.gainControllers) == 0 {
		gc2Config := agc.Config{
			Enabled: true,
			AdaptiveDigital: agc.AdaptiveDigitalConfig{
				Enabled:                  config.GainController2.AdaptiveDigital.Enabled,
				DryRun:                   config.GainController2.AdaptiveDigital.DryRun,
				HeadroomDb:               config.GainController2.AdaptiveDigital.HeadroomDb,
				MaxGainDb:                config.GainController2.AdaptiveDigital.MaxGainDb,
				InitialGainDb:            config.GainController2.AdaptiveDigital.InitialGainDb,
				MaxGainChangeDbPerSecond: config.GainController2.AdaptiveDigital.MaxGainChangeDbPerSecond,
				MaxOutputNoiseLevelDbfs:  config.GainController2.AdaptiveDigital.MaxOutputNoiseLevelDbfs,
			},
			FixedDigital: agc.FixedDigitalConfig{
				GainDb: config.GainController2.FixedDigital.GainDb,
			},
		}
		ap.gainControllers = make([]*agc.GainController2, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.gainControllers[ch] = agc.NewGainController2(gc2Config)
		}
	} else if !config.GainController2Enabled() {
		ap.gainControllers = nil
	}

	if config.EchoCancellerEnabled() && len(ap.echoCancellers) == 0 {
		ap.echoCancellers = make([]*aec3.EchoCanceller3, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.echoCancellers[ch] = aec3.NewEchoCanceller3(
				aec3.DefaultConfig(),
				ap.captureConfig.SampleRateHz,
				1,
			)
		}
	} else if !config.EchoCancellerEnabled() {
		ap.echoCancellers = nil
	}

	ap.config = config
	return nil
}

// Config returns a snapshot of the current processing configuration.
func (ap *AudioProcessing) Config() Config {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.config
}

// SetStreamDelayMs informs the echo canceller of the end-to-end render-to-capture delay in milliseconds.
// values outside [0, 500] are clamped to that range.
// accurate delay estimation improves echo cancellation quality significantly.
func (ap *AudioProcessing) SetStreamDelayMs(delayMs int) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	if delayMs < 0 {
		delayMs = 0
	}
	if delayMs > 500 {
		delayMs = 500
	}
	ap.streamDelayMs = delayMs
}

// Statistics returns a snapshot of the most recently computed processing metrics.
// pointer fields in AudioProcessingStats are nil when the corresponding stage is not active.
func (ap *AudioProcessing) Statistics() AudioProcessingStats {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.stats
}

// Close releases all processing stage resources and renders the instance inoperable.
// subsequent calls to any Process* method will operate with no active stages.
func (ap *AudioProcessing) Close() {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.highPassFilters = nil
	ap.noiseSuppressors = nil
	ap.gainControllers = nil
	ap.echoCancellers = nil
}

func (ap *AudioProcessing) updateStats(data [][]float32) {
	if len(data) == 0 || len(data[0]) == 0 {
		return
	}

	samples := data[0]
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	rms := math.Sqrt(sum / float64(len(samples)))
	ap.statsDbfs = 20 * math.Log10(rms/32768.0 + 1e-10)
	ap.stats.OutputRmsDbfs = &ap.statsDbfs

	if len(ap.echoCancellers) > 0 && ap.echoCancellers[0] != nil {
		ap.statsErle = float64(ap.echoCancellers[0].ERLE())
		ap.stats.EchoReturnLossEnhancement = &ap.statsErle
		ap.statsDelay = ap.echoCancellers[0].Delay()
		ap.stats.DelayMs = &ap.statsDelay
	}
}
