package sonora

import "fmt"

// Builder constructs an AudioProcessing instance using a fluent API.
// the zero value is not usable; create one with NewBuilder.
type Builder struct {
	captureConfig StreamConfig
	renderConfig  StreamConfig
	config        Config
}

// NewBuilder returns a Builder with defaults: 16 kHz, mono, all stages disabled.
func NewBuilder() *Builder {
	return &Builder{
		captureConfig: StreamConfig{SampleRateHz: SampleRate16kHz, NumChannels: 1},
		renderConfig:  StreamConfig{SampleRateHz: SampleRate16kHz, NumChannels: 1},
		config:        DefaultConfig(),
	}
}

// SampleRate sets the sample rate for both capture and render streams.
// use CaptureConfig or RenderConfig for asymmetric rates.
func (b *Builder) SampleRate(rate uint32) *Builder {
	b.captureConfig.SampleRateHz = rate
	b.renderConfig.SampleRateHz = rate
	return b
}

// Channels sets the channel count for both capture and render streams.
func (b *Builder) Channels(ch uint16) *Builder {
	b.captureConfig.NumChannels = ch
	b.renderConfig.NumChannels = ch
	return b
}

// CaptureConfig sets the stream configuration for the capture (microphone) path.
func (b *Builder) CaptureConfig(cfg StreamConfig) *Builder {
	b.captureConfig = cfg
	return b
}

// RenderConfig sets the stream configuration for the render (speaker) path.
// only required when echo cancellation is enabled.
func (b *Builder) RenderConfig(cfg StreamConfig) *Builder {
	b.renderConfig = cfg
	return b
}

// EnableHighPassFilter activates the high-pass filter stage with the given config.
func (b *Builder) EnableHighPassFilter(cfg HighPassFilterConfig) *Builder {
	b.config.HighPassFilter = &cfg
	return b
}

// EnablePreAmplifier activates the pre-amplifier stage with the given config.
func (b *Builder) EnablePreAmplifier(cfg PreAmplifierConfig) *Builder {
	b.config.PreAmplifier = &cfg
	return b
}

// EnableEchoCanceller activates AEC3 with the given config.
// render audio must be submitted via ProcessRender* each frame before capture.
func (b *Builder) EnableEchoCanceller(cfg EchoCancellerConfig) *Builder {
	b.config.EchoCanceller = &cfg
	return b
}

// EnableNoiseSuppression activates the noise suppressor with the given config.
func (b *Builder) EnableNoiseSuppression(cfg NsConfig) *Builder {
	b.config.NoiseSuppression = &cfg
	return b
}

// EnableGainController2 activates AGC2 with the given config.
// it forces Enabled to true regardless of the value in cfg.
func (b *Builder) EnableGainController2(cfg GainController2Config) *Builder {
	cfg.Enabled = true
	b.config.GainController2 = &cfg
	return b
}

// WithConfig replaces the entire processing config, overriding any Enable* calls made before it.
func (b *Builder) WithConfig(cfg Config) *Builder {
	b.config = cfg
	return b
}

// Build validates the stream configs and returns a ready-to-use AudioProcessing instance.
// returns an error if sample rate or channel count is out of range.
func (b *Builder) Build() (*AudioProcessing, error) {
	if err := b.captureConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid capture config: %w", err)
	}
	if err := b.renderConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid render config: %w", err)
	}
	return newAudioProcessing(b.captureConfig, b.renderConfig, b.config)
}
