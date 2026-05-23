package sonora

import (
	"math"

	"sonora/dsp"
)

type AudioBuffer struct {
	data            [][]float32
	splitData       [][][]float32
	numChannels     int
	numBands        int
	numFrames       int
	sampleRateHz    uint32
	splittingFilter *dsp.SplittingFilter
}

func NewAudioBuffer(config StreamConfig) *AudioBuffer {
	numBands := numBandsForRate(config.SampleRateHz)
	framesPerBand := int(config.SampleRateHz) / 100 / numBands
	channels := int(config.NumChannels)

	buf := &AudioBuffer{
		numChannels:  channels,
		numBands:     numBands,
		numFrames:    framesPerBand,
		sampleRateHz: config.SampleRateHz,
	}

	buf.data = make([][]float32, channels)
	for ch := 0; ch < channels; ch++ {
		buf.data[ch] = make([]float32, int(config.SampleRateHz)/100)
	}

	if numBands > 1 {
		buf.splitData = make([][][]float32, channels)
		for ch := 0; ch < channels; ch++ {
			buf.splitData[ch] = make([][]float32, numBands)
			for b := 0; b < numBands; b++ {
				buf.splitData[ch][b] = make([]float32, framesPerBand)
			}
		}
		buf.splittingFilter = dsp.NewSplittingFilter(channels, numBands)
	}

	return buf
}

func (ab *AudioBuffer) Channel(ch int) []float32 {
	return ab.data[ch]
}

func (ab *AudioBuffer) Channels() int {
	return ab.numChannels
}

func (ab *AudioBuffer) Bands() int {
	return ab.numBands
}

func (ab *AudioBuffer) FramesPerBand() int {
	return ab.numFrames
}

func (ab *AudioBuffer) SplitChannel(ch, band int) []float32 {
	if ab.numBands == 1 {
		return ab.data[ch]
	}
	return ab.splitData[ch][band]
}

func (ab *AudioBuffer) CopyFromInterleaved(src []int16) {
	frameSize := int(ab.sampleRateHz) / 100
	for ch := 0; ch < ab.numChannels; ch++ {
		for i := 0; i < frameSize; i++ {
			ab.data[ch][i] = float32(src[i*ab.numChannels+ch])
		}
	}
}

func (ab *AudioBuffer) CopyToInterleaved(dst []int16) {
	frameSize := int(ab.sampleRateHz) / 100
	for ch := 0; ch < ab.numChannels; ch++ {
		for i := 0; i < frameSize; i++ {
			s := float32(math.Round(float64(ab.data[ch][i])))
			if s > 32767 {
				s = 32767
			} else if s < -32768 {
				s = -32768
			}
			dst[i*ab.numChannels+ch] = int16(s)
		}
	}
}

func (ab *AudioBuffer) CopyFromFloat(src [][]float32) {
	for ch := 0; ch < ab.numChannels && ch < len(src); ch++ {
		copy(ab.data[ch], src[ch])
	}
}

func (ab *AudioBuffer) CopyToFloat(dst [][]float32) {
	for ch := 0; ch < ab.numChannels && ch < len(dst); ch++ {
		copy(dst[ch], ab.data[ch])
	}
}

func (ab *AudioBuffer) Clear() {
	for ch := 0; ch < ab.numChannels; ch++ {
		clear(ab.data[ch])
	}
	if ab.splitData != nil {
		for ch := 0; ch < ab.numChannels; ch++ {
			for b := 0; b < ab.numBands; b++ {
				clear(ab.splitData[ch][b])
			}
		}
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

func (ab *AudioBuffer) SplitIntoFrequencyBands() {
	if ab.splittingFilter != nil && ab.splitData != nil {
		ab.splittingFilter.Analysis(ab.data, ab.splitData)
	}
}

func (ab *AudioBuffer) MergeFrequencyBands() {
	if ab.splittingFilter != nil && ab.splitData != nil {
		ab.splittingFilter.Synthesis(ab.splitData, ab.data)
	}
}
