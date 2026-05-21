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

	highPassFilter  *HighPassFilter
	noiseSuppressor *ns.Suppressor
	gainController  *agc.GainController2
	echoCanceller   *aec3.EchoCanceller3

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

	if config.HighPassFilterEnabled() {
		ap.highPassFilter = NewHighPassFilter(captureConfig.SampleRateHz, int(captureConfig.NumChannels))
	}

	if config.NoiseSuppressionEnabled() {
		nsCfg := ns.Config{Level: ns.SuppressionLevel(config.NoiseSuppression.Level)}
		ap.noiseSuppressor = ns.NewSuppressor(nsCfg)
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
		ap.gainController = agc.NewGainController2(gc2Config)
	}

	if config.EchoCancellerEnabled() {
		ap.echoCanceller = aec3.NewEchoCanceller3(
			aec3.DefaultConfig(),
			captureConfig.SampleRateHz,
			int(captureConfig.NumChannels),
		)
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

	ap.captureBuffer.CopyFromFloat(data)

	if ap.highPassFilter != nil {
		ap.highPassFilter.Process(data)
	}

	if ap.echoCanceller != nil {
		for start := 0; start+aec3.BlockSize <= expectedFrames; start += aec3.BlockSize {
			block := data[0][start : start+aec3.BlockSize]
			ap.echoCanceller.ProcessCapture(block)
		}
	}

	if ap.noiseSuppressor != nil {
		ap.noiseSuppressor.Process(data[0])
	}

	if ap.gainController != nil {
		ap.gainController.Process(data[0])
	}

	ap.updateStats(data[0])
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

	if ap.echoCanceller != nil {
		expectedFrames := ap.renderConfig.FrameSize()
		for start := 0; start+aec3.BlockSize <= expectedFrames; start += aec3.BlockSize {
			block := data[0][start : start+aec3.BlockSize]
			ap.echoCanceller.ProcessRender(block)
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

	if config.HighPassFilterEnabled() && ap.highPassFilter == nil {
		ap.highPassFilter = NewHighPassFilter(ap.captureConfig.SampleRateHz, int(ap.captureConfig.NumChannels))
	} else if !config.HighPassFilterEnabled() {
		ap.highPassFilter = nil
	}

	if config.NoiseSuppressionEnabled() {
		nsCfg := ns.Config{Level: ns.SuppressionLevel(config.NoiseSuppression.Level)}
		ap.noiseSuppressor = ns.NewSuppressor(nsCfg)
	} else {
		ap.noiseSuppressor = nil
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
	ap.highPassFilter = nil
	ap.noiseSuppressor = nil
	ap.gainController = nil
	ap.echoCanceller = nil
}

func (ap *AudioProcessing) updateStats(samples []float32) {
	if len(samples) == 0 {
		return
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	rms := math.Sqrt(sum / float64(len(samples)))
	dbfs := 20 * math.Log10(rms+1e-10)
	ap.stats.OutputRmsDbfs = &dbfs

	if ap.echoCanceller != nil {
		erle := float64(ap.echoCanceller.ERLE())
		ap.stats.EchoReturnLossEnhancement = &erle
		delay := ap.echoCanceller.Delay()
		ap.stats.DelayMs = &delay
	}
}
