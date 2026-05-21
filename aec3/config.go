// Package aec3 implements Acoustic Echo Cancellation (AEC3)
// based on WebRTC's AEC3 algorithm. It processes audio in 64-sample
// blocks using frequency-domain adaptive filtering to remove echo
// from the capture signal.
package aec3

const (
	BlockSize            = 64
	SubFrameLength       = 80
	FFTSize              = 128
	FFTSizeBy2Plus1      = FFTSize/2 + 1
	NumBlocksPerSecond   = 250
	SubFramesPerFrame    = 2
	BlocksPerSubFrame    = 4
)

type EchoCanceller3Config struct {
	Delay            DelayConfig
	Filter           FilterConfig
	Erle             ErleConfig
	EchoAudibility   EchoAudibilityConfig
	Suppressor       SuppressorConfig
}

type DelayConfig struct {
	DefaultDelayBlocks int
	DownSamplingFactor int
	NumFilters         int
	DelayHeadroomBlocks int
	HysteresisLimitBlocks int
}

type FilterConfig struct {
	Refined FilterPartConfig
	Coarse  FilterPartConfig
}

type FilterPartConfig struct {
	LengthBlocks  int
	RateBlocks    int
	InitialScale  float32
	ErrorFloorLog2 float32
}

type ErleConfig struct {
	Min       float32
	MaxLf     float32
	MaxHf     float32
	OnsetRate float32
}

type EchoAudibilityConfig struct {
	LowRenderLimit  float32
	NormalRenderLimit float32
}

type SuppressorConfig struct {
	NormalTuning    SuppressionTuning
	NearendTuning   SuppressionTuning
}

type SuppressionTuning struct {
	MaskLf        MaskConfig
	MaskHf        MaskConfig
	MaxIncFactor  float32
	MaxDecFactor  float32
}

type MaskConfig struct {
	EnrTransparent float32
	EnrSuppress    float32
	EmrTransparent float32
}

func DefaultConfig() EchoCanceller3Config {
	return EchoCanceller3Config{
		Delay: DelayConfig{
			DefaultDelayBlocks:    5,
			DownSamplingFactor:    4,
			NumFilters:            6,
			DelayHeadroomBlocks:   2,
			HysteresisLimitBlocks: 1,
		},
		Filter: FilterConfig{
			Refined: FilterPartConfig{
				LengthBlocks:   13,
				RateBlocks:     250,
				InitialScale:   0.01,
				ErrorFloorLog2: -10,
			},
			Coarse: FilterPartConfig{
				LengthBlocks:   13,
				RateBlocks:     250,
				InitialScale:   0.001,
				ErrorFloorLog2: -10,
			},
		},
		Erle: ErleConfig{
			Min:       1.0,
			MaxLf:     8.0,
			MaxHf:     1.5,
			OnsetRate: 0.1,
		},
		EchoAudibility: EchoAudibilityConfig{
			LowRenderLimit:    4 * 64,
			NormalRenderLimit:  64,
		},
		Suppressor: SuppressorConfig{
			NormalTuning: SuppressionTuning{
				MaskLf:       MaskConfig{EnrTransparent: 0.3, EnrSuppress: 0.8, EmrTransparent: 0.4},
				MaskHf:       MaskConfig{EnrTransparent: 0.07, EnrSuppress: 0.1, EmrTransparent: 0.3},
				MaxIncFactor: 2.0,
				MaxDecFactor: 0.5,
			},
			NearendTuning: SuppressionTuning{
				MaskLf:       MaskConfig{EnrTransparent: 1.09, EnrSuppress: 1.1, EmrTransparent: 0.3},
				MaskHf:       MaskConfig{EnrTransparent: 0.1, EnrSuppress: 0.3, EmrTransparent: 0.3},
				MaxIncFactor: 2.0,
				MaxDecFactor: 0.5,
			},
		},
	}
}

func numBandsForRate(sampleRate uint32) int {
	switch {
	case sampleRate <= 16000:
		return 1
	case sampleRate <= 32000:
		return 2
	default:
		return 3
	}
}
