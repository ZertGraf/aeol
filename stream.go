package aeol

import "fmt"

// Supported sample rate constants. AEC3 and NS always operate at 16 kHz internally;
// higher rates are downsampled to the lowest sub-band via the splitting filter.
const (
	SampleRate16kHz = 16000
	SampleRate32kHz = 32000
	SampleRate48kHz = 48000

	// MaxSampleRate is the upper bound accepted by NewStreamConfig and Validate.
	MaxSampleRate = 384000
	// MinSampleRate is the lower bound accepted by NewStreamConfig and Validate.
	MinSampleRate = SampleRate16kHz

	// MaxChannels is the maximum number of channels per stream.
	MaxChannels = 8
)

// StreamConfig describes the format of an audio stream passed to AudioProcessing.
// all processing operates on exactly one 10 ms frame at a time.
type StreamConfig struct {
	SampleRateHz uint32
	NumChannels  uint16
}

// NewStreamConfig constructs a StreamConfig and validates the rate and channel count.
// returns an error if either value is out of the supported range.
func NewStreamConfig(sampleRate uint32, channels uint16) (StreamConfig, error) {
	if sampleRate < MinSampleRate || sampleRate > MaxSampleRate {
		return StreamConfig{}, fmt.Errorf("sample rate %d out of range [%d, %d]", sampleRate, MinSampleRate, MaxSampleRate)
	}
	if channels == 0 || channels > MaxChannels {
		return StreamConfig{}, fmt.Errorf("channels %d out of range [1, %d]", channels, MaxChannels)
	}
	return StreamConfig{SampleRateHz: sampleRate, NumChannels: channels}, nil
}

// FrameSize returns the number of samples per channel in one 10 ms frame.
func (sc StreamConfig) FrameSize() int {
	return int(sc.SampleRateHz) / 100
}

// SamplesPerChannel returns the number of samples each channel contributes per frame.
// equivalent to FrameSize.
func (sc StreamConfig) SamplesPerChannel() int {
	return sc.FrameSize()
}

// TotalSamples returns the total number of samples in one interleaved frame across all channels.
func (sc StreamConfig) TotalSamples() int {
	return sc.FrameSize() * int(sc.NumChannels)
}

// Validate checks that SampleRateHz and NumChannels are within the supported ranges.
func (sc StreamConfig) Validate() error {
	if sc.SampleRateHz < MinSampleRate || sc.SampleRateHz > MaxSampleRate {
		return fmt.Errorf("invalid sample rate: %d", sc.SampleRateHz)
	}
	if sc.NumChannels == 0 || sc.NumChannels > MaxChannels {
		return fmt.Errorf("invalid channel count: %d", sc.NumChannels)
	}
	return nil
}


// ToFloatS16 converts normalized [-1, 1] samples to FloatS16 [-32768, 32767] in place.
func ToFloatS16(samples []float32) {
	for i := range samples {
		samples[i] *= 32768.0
	}
}

// FromFloatS16 converts FloatS16 [-32768, 32767] samples to normalized [-1, 1] in place.
func FromFloatS16(samples []float32) {
	for i := range samples {
		samples[i] /= 32768.0
	}
}
