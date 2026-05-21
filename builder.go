package sonora

import "fmt"

type Builder struct {
	captureConfig StreamConfig
	renderConfig  StreamConfig
	config        Config
}

func NewBuilder() *Builder {
	return &Builder{
		captureConfig: StreamConfig{SampleRateHz: SampleRate16kHz, NumChannels: 1},
		renderConfig:  StreamConfig{SampleRateHz: SampleRate16kHz, NumChannels: 1},
		config:        DefaultConfig(),
	}
}

func (b *Builder) SampleRate(rate uint32) *Builder {
	b.captureConfig.SampleRateHz = rate
	b.renderConfig.SampleRateHz = rate
	return b
}

func (b *Builder) Channels(ch uint16) *Builder {
	b.captureConfig.NumChannels = ch
	b.renderConfig.NumChannels = ch
	return b
}

func (b *Builder) CaptureConfig(cfg StreamConfig) *Builder {
	b.captureConfig = cfg
	return b
}

func (b *Builder) RenderConfig(cfg StreamConfig) *Builder {
	b.renderConfig = cfg
	return b
}

func (b *Builder) EnableHighPassFilter(cfg HighPassFilterConfig) *Builder {
	b.config.HighPassFilter = &cfg
	return b
}

func (b *Builder) EnablePreAmplifier(cfg PreAmplifierConfig) *Builder {
	b.config.PreAmplifier = &cfg
	return b
}

func (b *Builder) EnableEchoCanceller(cfg EchoCancellerConfig) *Builder {
	b.config.EchoCanceller = &cfg
	return b
}

func (b *Builder) EnableNoiseSuppression(cfg NsConfig) *Builder {
	b.config.NoiseSuppression = &cfg
	return b
}

func (b *Builder) EnableGainController2(cfg GainController2Config) *Builder {
	cfg.Enabled = true
	b.config.GainController2 = &cfg
	return b
}

func (b *Builder) WithConfig(cfg Config) *Builder {
	b.config = cfg
	return b
}

func (b *Builder) Build() (*AudioProcessing, error) {
	if err := b.captureConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid capture config: %w", err)
	}
	if err := b.renderConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid render config: %w", err)
	}
	return newAudioProcessing(b.captureConfig, b.renderConfig, b.config)
}
