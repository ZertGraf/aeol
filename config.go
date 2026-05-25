package aeol

// NsLevel controls the aggressiveness of the noise suppressor.
// higher values remove more noise but may also suppress speech.
type NsLevel int

const (
	// NsLevelLow applies minimal noise suppression, preserving more of the original signal.
	NsLevelLow NsLevel = 0
	// NsLevelModerate applies a balanced amount of suppression suitable for most environments.
	NsLevelModerate NsLevel = 1
	// NsLevelHigh aggressively suppresses noise; may reduce speech quality in clean conditions.
	NsLevelHigh NsLevel = 2
	// NsLevelVeryHigh applies maximum suppression; use only in very noisy environments.
	NsLevelVeryHigh NsLevel = 3
)

// HighPassFilterConfig configures the high-pass filter applied to the capture signal.
// the filter removes low-frequency rumble and DC offset below roughly 80 Hz.
type HighPassFilterConfig struct{}

// DefaultHighPassFilterConfig returns a HighPassFilterConfig with default settings.
func DefaultHighPassFilterConfig() HighPassFilterConfig {
	return HighPassFilterConfig{}
}

// PreAmplifierConfig applies a linear gain to the capture signal before any other processing.
// Gain is a linear multiplier; 1.0 means no change, 2.0 doubles amplitude.
type PreAmplifierConfig struct {
	Gain float32
}

// DefaultPreAmplifierConfig returns a PreAmplifierConfig with unity gain (no amplification).
func DefaultPreAmplifierConfig() PreAmplifierConfig {
	return PreAmplifierConfig{Gain: 1.0}
}

// CaptureLevelAdjustmentConfig applies dB gain before and after the main processing chain.
// PreGainDb is applied before AEC/NS, PostGainDb is applied after AGC2.
type CaptureLevelAdjustmentConfig struct {
	Enabled    bool
	PreGainDb  float32
	PostGainDb float32
}

// DefaultCaptureLevelAdjustmentConfig returns a CaptureLevelAdjustmentConfig with all adjustments disabled.
func DefaultCaptureLevelAdjustmentConfig() CaptureLevelAdjustmentConfig {
	return CaptureLevelAdjustmentConfig{
		Enabled:    false,
		PreGainDb:  0.0,
		PostGainDb: 0.0,
	}
}

// EchoCancellerConfig configures the AEC3 acoustic echo canceller.
// the orchestrator uses default AEC3 parameters; for fine-tuning (delay,
// filter length, ERLE bounds, suppressor masks) use the standalone
// aec3.NewEchoCanceller3 API with a custom aec3.EchoCanceller3Config.
type EchoCancellerConfig struct{}

// DefaultEchoCancellerConfig returns an EchoCancellerConfig with default settings.
func DefaultEchoCancellerConfig() EchoCancellerConfig {
	return EchoCancellerConfig{}
}

// NsConfig configures the noise suppressor.
type NsConfig struct {
	// Level sets the suppression aggressiveness; see NsLevel constants.
	Level NsLevel
}

// DefaultNsConfig returns a NsConfig at moderate suppression level.
func DefaultNsConfig() NsConfig {
	return NsConfig{Level: NsLevelModerate}
}

// GainController2Config configures AGC2, which normalizes the output level after NS.
// set Enabled to true to activate; the controller is skipped when Enabled is false.
type GainController2Config struct {
	Enabled         bool
	AdaptiveDigital AdaptiveDigitalConfig
	FixedDigital    FixedDigitalConfig
}

// AdaptiveDigitalConfig tunes the adaptive digital gain stage within AGC2.
// HeadroomDb reserves dB of headroom below 0 dBFS to prevent clipping.
// MaxGainDb caps the total gain applied to the signal.
// MaxGainChangeDbPerSecond limits how fast the gain ramps up or down.
// MaxOutputNoiseLevelDbfs suppresses gain increases when noise floor exceeds this threshold.
type AdaptiveDigitalConfig struct {
	Enabled       bool
	DryRun        bool
	HeadroomDb    float32
	MaxGainDb     float32
	InitialGainDb float32
	MaxGainChangeDbPerSecond float32
	MaxOutputNoiseLevelDbfs  float32
}

// FixedDigitalConfig applies a constant dB gain after the adaptive stage.
// GainDb of 0 means no additional gain.
type FixedDigitalConfig struct {
	GainDb float32
}

// DefaultGainController2Config returns a GainController2Config with adaptive digital gain enabled
// and conservative defaults suitable for most microphone inputs.
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
	}
}

// Config is the top-level configuration for an AudioProcessing instance.
// each field is a pointer; a nil pointer disables the corresponding stage entirely.
type Config struct {
	PreAmplifier           *PreAmplifierConfig
	CaptureLevelAdjustment *CaptureLevelAdjustmentConfig
	HighPassFilter         *HighPassFilterConfig
	EchoCanceller          *EchoCancellerConfig
	NoiseSuppression       *NsConfig
	GainController2        *GainController2Config
}

// DefaultConfig returns an empty Config with all processing stages disabled.
func DefaultConfig() Config {
	return Config{}
}

// EchoCancellerEnabled reports whether the echo canceller stage is configured.
func (c Config) EchoCancellerEnabled() bool {
	return c.EchoCanceller != nil
}

// NoiseSuppressionEnabled reports whether the noise suppression stage is configured.
func (c Config) NoiseSuppressionEnabled() bool {
	return c.NoiseSuppression != nil
}

// GainController2Enabled reports whether AGC2 is configured and explicitly enabled.
func (c Config) GainController2Enabled() bool {
	return c.GainController2 != nil && c.GainController2.Enabled
}

// HighPassFilterEnabled reports whether the high-pass filter stage is configured.
func (c Config) HighPassFilterEnabled() bool {
	return c.HighPassFilter != nil
}

// PreAmplifierEnabled reports whether the pre-amplifier stage is configured.
func (c Config) PreAmplifierEnabled() bool {
	return c.PreAmplifier != nil
}

// NeedsRender reports whether the current config requires render (far-end) audio to be provided.
// render audio must be submitted via ProcessRender* before the corresponding capture frame.
func (c Config) NeedsRender() bool {
	return c.EchoCancellerEnabled()
}
