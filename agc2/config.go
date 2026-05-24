// Package agc2 implements Automatic Gain Control (AGC2) based on WebRTC's
// AGC2 algorithm. It provides adaptive digital gain control with speech
// level estimation, noise floor tracking, and output limiting.
//
// AGC2 operates on full-band audio (no band splitting needed).
// It accepts 10ms frames at any supported sample rate.
//
// All samples are in FloatS16 format (float32 in [-32768, 32767]).
//
// Instances are not safe for concurrent use; synchronization is the caller's responsibility.
package agc2

import "math"

const (
	frameDurationMs = 10
	sampleRate16k   = 16000
	framesPerSecond = 1000 / frameDurationMs

	dbfsMax    = 0.0
	dbfsMin    = -90.0
	maxGainDb  = 30.0
	minGainDb  = 0.0
	minLevelDb = -90.0
)

// Config is the top-level AGC2 configuration. set Enabled to false to bypass the entire stage.
type Config struct {
	Enabled         bool
	AdaptiveDigital AdaptiveDigitalConfig
	FixedDigital    FixedDigitalConfig
}

// AdaptiveDigitalConfig controls the speech-level-tracking adaptive gain stage.
// DryRun runs all estimators without modifying samples, useful for tuning.
// HeadroomDb is subtracted from the computed target gain before application.
// MaxGainChangeDbPerSecond limits how fast the gain ramps up or down.
// MaxOutputNoiseLevelDbfs prevents boosting noise above this dBFS ceiling.
type AdaptiveDigitalConfig struct {
	Enabled                  bool
	DryRun                   bool
	HeadroomDb               float32
	MaxGainDb                float32
	InitialGainDb            float32
	MaxGainChangeDbPerSecond float32
	MaxOutputNoiseLevelDbfs  float32
}

// FixedDigitalConfig applies a constant linear gain derived from GainDb.
// GainDb of 0 means unity gain (no change).
type FixedDigitalConfig struct {
	GainDb float32
}

// DefaultConfig returns a recommended configuration targeting -18 dBFS speech level,
// with 30 dB max gain, 3 dB/s ramp rate, and -50 dBFS noise ceiling.
func DefaultConfig() Config {
	return Config{
		Enabled: true,
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

func dbToLinear(db float32) float32 {
	return float32(math.Pow(10.0, float64(db)/20.0))
}

func linearToDb(linear float32) float32 {
	if linear <= 0 {
		return -100
	}
	return float32(20.0 * math.Log10(float64(linear) / 32768.0))
}
