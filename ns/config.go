// Package ns implements noise suppression based on the WebRTC NS algorithm.
//
// The algorithm operates on 10ms frames at 16kHz (160 samples) using a
// 256-point FFT with 129 frequency bins. It estimates the noise spectrum
// using quantile tracking and applies Wiener filtering to suppress noise
// while preserving speech.
package ns

const (
	fftSize       = 256
	numFreqBins   = fftSize/2 + 1
	frameLength   = 160
	analysisSize  = fftSize
	overlapSize   = fftSize - frameLength
	blockSize10ms = 160
)

type SuppressionLevel int

const (
	SuppressionLow      SuppressionLevel = 0
	SuppressionModerate SuppressionLevel = 1
	SuppressionHigh     SuppressionLevel = 2
	SuppressionVeryHigh SuppressionLevel = 3
)

type Config struct {
	Level SuppressionLevel
}

func DefaultConfig() Config {
	return Config{Level: SuppressionModerate}
}

type suppressionParams struct {
	overSubtractionFactor float32
	minOverDrive          float32
}

func getSuppressionParams(level SuppressionLevel) suppressionParams {
	switch level {
	case SuppressionLow:
		return suppressionParams{1.0, 1.0}
	case SuppressionModerate:
		return suppressionParams{1.0, 1.2}
	case SuppressionHigh:
		return suppressionParams{1.1, 1.5}
	case SuppressionVeryHigh:
		return suppressionParams{1.3, 2.0}
	default:
		return suppressionParams{1.0, 1.2}
	}
}
