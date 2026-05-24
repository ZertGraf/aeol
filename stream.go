package sonora

import (
	"errors"
	"fmt"
)

const (
	SampleRate8kHz   = 8000
	SampleRate16kHz  = 16000
	SampleRate32kHz  = 32000
	SampleRate48kHz  = 48000

	MaxSampleRate = 384000
	MinSampleRate = SampleRate8kHz

	MaxChannels = 8
)

type StreamConfig struct {
	SampleRateHz uint32
	NumChannels  uint16
}

func NewStreamConfig(sampleRate uint32, channels uint16) (StreamConfig, error) {
	if sampleRate < MinSampleRate || sampleRate > MaxSampleRate {
		return StreamConfig{}, fmt.Errorf("sample rate %d out of range [%d, %d]", sampleRate, MinSampleRate, MaxSampleRate)
	}
	if channels == 0 || channels > MaxChannels {
		return StreamConfig{}, fmt.Errorf("channels %d out of range [1, %d]", channels, MaxChannels)
	}
	return StreamConfig{SampleRateHz: sampleRate, NumChannels: channels}, nil
}

func (sc StreamConfig) FrameSize() int {
	return int(sc.SampleRateHz) / 100
}

func (sc StreamConfig) SamplesPerChannel() int {
	return sc.FrameSize()
}

func (sc StreamConfig) TotalSamples() int {
	return sc.FrameSize() * int(sc.NumChannels)
}

func (sc StreamConfig) Validate() error {
	if sc.SampleRateHz < MinSampleRate || sc.SampleRateHz > MaxSampleRate {
		return fmt.Errorf("invalid sample rate: %d", sc.SampleRateHz)
	}
	if sc.NumChannels == 0 || sc.NumChannels > MaxChannels {
		return fmt.Errorf("invalid channel count: %d", sc.NumChannels)
	}
	return nil
}

var ErrInvalidStreamConfig = errors.New("invalid stream config")

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
