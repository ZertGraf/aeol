package sonora

type NsLevel int

const (
	NsLevelLow      NsLevel = 0
	NsLevelModerate NsLevel = 1
	NsLevelHigh     NsLevel = 2
	NsLevelVeryHigh NsLevel = 3
)

type HighPassFilterConfig struct {
	ApplyInFullBand bool
}

func DefaultHighPassFilterConfig() HighPassFilterConfig {
	return HighPassFilterConfig{ApplyInFullBand: true}
}

type PreAmplifierConfig struct {
	Gain float32
}

func DefaultPreAmplifierConfig() PreAmplifierConfig {
	return PreAmplifierConfig{Gain: 1.0}
}

type CaptureLevelAdjustmentConfig struct {
	Enabled      bool
	PreGainDb    float32
	PostGainDb   float32
	AnalogMicGainEmulation AnalogMicGainEmulationConfig
}

type AnalogMicGainEmulationConfig struct {
	Enabled    bool
	InitLevel  int
	MinLevel   int
	MaxLevel   int
}

func DefaultCaptureLevelAdjustmentConfig() CaptureLevelAdjustmentConfig {
	return CaptureLevelAdjustmentConfig{
		Enabled:    false,
		PreGainDb:  0.0,
		PostGainDb: 0.0,
		AnalogMicGainEmulation: AnalogMicGainEmulationConfig{
			Enabled:   false,
			InitLevel: 255,
			MinLevel:  12,
			MaxLevel:  255,
		},
	}
}

type EchoCancellerConfig struct {
	EnforceHighPassFiltering bool
}

func DefaultEchoCancellerConfig() EchoCancellerConfig {
	return EchoCancellerConfig{EnforceHighPassFiltering: true}
}

type NsConfig struct {
	Level NsLevel
}

func DefaultNsConfig() NsConfig {
	return NsConfig{Level: NsLevelModerate}
}

type GainController2Config struct {
	Enabled              bool
	AdaptiveDigital      AdaptiveDigitalConfig
	FixedDigital         FixedDigitalConfig
	InputVolumeControl   InputVolumeControlConfig
}

type AdaptiveDigitalConfig struct {
	Enabled       bool
	DryRun        bool
	HeadroomDb    float32
	MaxGainDb     float32
	InitialGainDb float32
	MaxGainChangeDbPerSecond float32
	MaxOutputNoiseLevelDbfs  float32
}

type FixedDigitalConfig struct {
	GainDb float32
}

type InputVolumeControlConfig struct {
	Enabled bool
}

func DefaultGainController2Config() GainController2Config {
	return GainController2Config{
		Enabled: false,
		AdaptiveDigital: AdaptiveDigitalConfig{
			Enabled:                  true,
			DryRun:                   false,
			HeadroomDb:               1.0,
			MaxGainDb:                30.0,
			InitialGainDb:            8.0,
			MaxGainChangeDbPerSecond: 3.0,
			MaxOutputNoiseLevelDbfs:  -50.0,
		},
		FixedDigital: FixedDigitalConfig{GainDb: 0.0},
		InputVolumeControl: InputVolumeControlConfig{Enabled: false},
	}
}

type PipelineConfig struct {
	MaximumInternalProcessingRate uint32
	MultiChannelRender           bool
	MultiChannelCapture          bool
}

func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		MaximumInternalProcessingRate: SampleRate48kHz,
		MultiChannelRender:           false,
		MultiChannelCapture:          false,
	}
}

type Config struct {
	Pipeline              *PipelineConfig
	PreAmplifier          *PreAmplifierConfig
	CaptureLevelAdjustment *CaptureLevelAdjustmentConfig
	HighPassFilter        *HighPassFilterConfig
	EchoCanceller         *EchoCancellerConfig
	NoiseSuppression      *NsConfig
	GainController2       *GainController2Config
}

func DefaultConfig() Config {
	return Config{}
}

func (c Config) EchoCancellerEnabled() bool {
	return c.EchoCanceller != nil
}

func (c Config) NoiseSuppressionEnabled() bool {
	return c.NoiseSuppression != nil
}

func (c Config) GainController2Enabled() bool {
	return c.GainController2 != nil && c.GainController2.Enabled
}

func (c Config) HighPassFilterEnabled() bool {
	return c.HighPassFilter != nil
}

func (c Config) PreAmplifierEnabled() bool {
	return c.PreAmplifier != nil
}

func (c Config) NeedsRender() bool {
	return c.EchoCancellerEnabled()
}
