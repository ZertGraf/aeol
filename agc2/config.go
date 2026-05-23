// Package agc2 implements Automatic Gain Control (AGC2) based on WebRTC's
// AGC2 algorithm. It provides adaptive digital gain control with speech
// level estimation, noise floor tracking, and output limiting.
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

type Config struct {
	Enabled         bool
	AdaptiveDigital AdaptiveDigitalConfig
	FixedDigital    FixedDigitalConfig
}

type AdaptiveDigitalConfig struct {
	Enabled                  bool
	DryRun                   bool
	HeadroomDb               float32
	MaxGainDb                float32
	InitialGainDb            float32
	MaxGainChangeDbPerSecond float32
	MaxOutputNoiseLevelDbfs  float32
}

type FixedDigitalConfig struct {
	GainDb float32
}

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
