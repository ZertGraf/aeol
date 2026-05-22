package sonora

import (
	"fmt"
	"math"
	"sync"

	"sonora/aec3"
	agc "sonora/agc2"
	"sonora/ns"
)

type AudioProcessing struct {
	mu sync.Mutex

	captureConfig StreamConfig
	renderConfig  StreamConfig
	config        Config

	highPassFilters  []*HighPassFilter
	noiseSuppressors []*ns.Suppressor
	gainControllers  []*agc.GainController2
	echoCancellers   []*aec3.EchoCanceller3

	captureBuffer *AudioBuffer
	renderBuffer  *AudioBuffer

	streamDelayMs int
	stats         AudioProcessingStats
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
		ap.highPassFilters = make([]*HighPassFilter, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.highPassFilters[ch] = NewHighPassFilter(captureConfig.SampleRateHz, 1)
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

func (ap *AudioProcessing) ProcessCaptureFloat(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.processCaptureFloatLocked(data)
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
			ap.highPassFilters[ch].Process([][]float32{chData})
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

func (ap *AudioProcessing) ProcessRenderFloat(data [][]float32) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.processRenderFloatLocked(data)
}

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

func (ap *AudioProcessing) ApplyConfig(config Config) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	numChannels := int(ap.captureConfig.NumChannels)

	if config.HighPassFilterEnabled() && len(ap.highPassFilters) == 0 {
		ap.highPassFilters = make([]*HighPassFilter, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.highPassFilters[ch] = NewHighPassFilter(ap.captureConfig.SampleRateHz, 1)
		}
	} else if !config.HighPassFilterEnabled() {
		ap.highPassFilters = nil
	}

	if config.NoiseSuppressionEnabled() {
		nsCfg := ns.Config{Level: ns.SuppressionLevel(config.NoiseSuppression.Level)}
		ap.noiseSuppressors = make([]*ns.Suppressor, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			ap.noiseSuppressors[ch] = ns.NewSuppressor(nsCfg)
		}
	} else {
		ap.noiseSuppressors = nil
	}

	ap.config = config
	return nil
}

func (ap *AudioProcessing) Config() Config {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.config
}

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

func (ap *AudioProcessing) Statistics() AudioProcessingStats {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.stats
}

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
	dbfs := 20 * math.Log10(rms+1e-10)
	ap.stats.OutputRmsDbfs = &dbfs

	if len(ap.echoCancellers) > 0 && ap.echoCancellers[0] != nil {
		erle := float64(ap.echoCancellers[0].ERLE())
		ap.stats.EchoReturnLossEnhancement = &erle
		delay := ap.echoCancellers[0].Delay()
		ap.stats.DelayMs = &delay
	}
}
