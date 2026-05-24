// Package aec3 implements Acoustic Echo Cancellation (AEC3)
// based on WebRTC's AEC3 algorithm. It processes audio in 64-sample
// blocks using frequency-domain adaptive filtering to remove echo
// from the capture signal.
//
// Like ns, AEC3 operates on the 0-8kHz band only (16kHz internal rate).
// For higher sample rates, the caller must band-split first.
//
// All samples are in FloatS16 format (float32 in [-32768, 32767]).
//
// Instances are not safe for concurrent use; synchronization is the caller's responsibility.
package aec3

const (
	// BlockSize is the number of FloatS16 samples processed per AEC3 step.
	BlockSize = 64
	// SubFrameLength is the length of a sub-frame used by the frame blocker, in samples.
	SubFrameLength = 80
	// FFTSize is the transform length used throughout the package; the input is zero-padded to this size.
	FFTSize = 128
	// FFTSizeBy2Plus1 is the number of unique complex bins in a real FFT of size FFTSize.
	FFTSizeBy2Plus1 = FFTSize/2 + 1
	// NumBlocksPerSecond is the number of 64-sample blocks per second at 16 kHz.
	NumBlocksPerSecond = 250
	// SubFramesPerFrame is the number of sub-frames assembled into one processing frame.
	SubFramesPerFrame = 2
	// BlocksPerSubFrame is the number of 64-sample blocks contained in one sub-frame.
	BlocksPerSubFrame = 4
)

// EchoCanceller3Config holds all tunable parameters for the AEC3 engine.
// Use DefaultConfig to obtain a reasonable starting point.
type EchoCanceller3Config struct {
	Delay          DelayConfig
	Filter         FilterConfig
	Erle           ErleConfig
	EchoAudibility EchoAudibilityConfig
	Suppressor     SuppressorConfig
}

// DelayConfig controls the echo-path delay estimation.
// DefaultDelayBlocks is the assumed delay before estimation converges.
// DownSamplingFactor reduces sample rate for correlation search.
type DelayConfig struct {
	DefaultDelayBlocks    int
	DownSamplingFactor    int
	NumFilters            int
	DelayHeadroomBlocks   int
	HysteresisLimitBlocks int
}

// FilterConfig holds parameters for both the refined and coarse adaptive filters.
// CoarseResetHangoverBlocks controls how long the coarse filter stays frozen after a reset.
type FilterConfig struct {
	Refined                   FilterPartConfig
	Coarse                    FilterPartConfig
	CoarseResetHangoverBlocks int
}

// FilterPartConfig configures one adaptive filter (refined or coarse).
// LengthBlocks is the filter impulse-response length in 64-sample blocks.
// InitialScale is the NLMS step-size numerator before power normalization.
type FilterPartConfig struct {
	LengthBlocks   int
	RateBlocks     int
	InitialScale   float32
	ErrorFloorLog2 float32
}

// ErleConfig bounds the echo return loss enhancement estimate.
// Min and MaxLf/MaxHf cap the instantaneous ERLE update in linear scale.
type ErleConfig struct {
	Min       float32
	MaxLf     float32
	MaxHf     float32
	OnsetRate float32
}

// EchoAudibilityConfig sets render-signal power thresholds below which
// echo is considered inaudible and suppression is relaxed.
type EchoAudibilityConfig struct {
	LowRenderLimit    float32
	NormalRenderLimit float32
}

// SuppressorConfig holds frequency-domain gain tuning for normal and nearend conditions.
type SuppressorConfig struct {
	NormalTuning  SuppressionTuning
	NearendTuning SuppressionTuning
}

// SuppressionTuning controls per-band gain floors and rate-of-change limits.
// MaskLf applies to bins 0-32; MaskHf applies to bins 33-64.
type SuppressionTuning struct {
	MaskLf       MaskConfig
	MaskHf       MaskConfig
	MaxIncFactor float32
	MaxDecFactor float32
}

// MaskConfig defines the gain floor (EnrTransparent) and suppression knee (EnrSuppress)
// for one frequency band in terms of echo-to-nearend energy ratio.
type MaskConfig struct {
	EnrTransparent float32
	EnrSuppress    float32
	EmrTransparent float32
}

// DefaultConfig returns a EchoCanceller3Config with WebRTC-derived defaults.
// Suitable for 16 kHz mono capture with moderate echo levels.
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
				InitialScale:   0.05,
				ErrorFloorLog2: -10,
			},
			CoarseResetHangoverBlocks: 25,
		},
		Erle: ErleConfig{
			Min:       1.0,
			MaxLf:     4.0,
			MaxHf:     1.5,
			OnsetRate: 0.1,
		},
		EchoAudibility: EchoAudibilityConfig{
			LowRenderLimit:    4 * 64,
			NormalRenderLimit:  64,
		},
		Suppressor: SuppressorConfig{
			NormalTuning: SuppressionTuning{
				MaskLf:       MaskConfig{EnrTransparent: 0.3, EnrSuppress: 0.4, EmrTransparent: 0.4},
				MaskHf:       MaskConfig{EnrTransparent: 0.07, EnrSuppress: 0.1, EmrTransparent: 0.3},
				MaxIncFactor: 2.0,
				MaxDecFactor: 0.25,
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
