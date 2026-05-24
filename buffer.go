package sonora

import (
	"math"

	"sonora/dsp"
)

// AudioBuffer holds one 10 ms frame of audio in de-interleaved, FloatS16 format.
// when the sample rate exceeds 16 kHz, it also maintains per-band sub-buffers
// populated by SplitIntoFrequencyBands and consumed by MergeFrequencyBands.
type AudioBuffer struct {
	data            [][]float32
	splitData       [][][]float32
	numChannels     int
	numBands        int
	numFrames       int
	sampleRateHz    uint32
	splittingFilter *dsp.SplittingFilter
}

// NewAudioBuffer allocates an AudioBuffer sized for the given stream configuration.
// band splitting structures are only allocated when the sample rate requires more than one band.
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

// Channel returns the full-band sample slice for the given channel index in FloatS16 format.
func (ab *AudioBuffer) Channel(ch int) []float32 {
	return ab.data[ch]
}

// Channels returns the number of audio channels in this buffer.
func (ab *AudioBuffer) Channels() int {
	return ab.numChannels
}

// Bands returns the number of frequency sub-bands: 1 at ≤16 kHz, 2 at ≤32 kHz, 3 at 48 kHz.
func (ab *AudioBuffer) Bands() int {
	return ab.numBands
}

// FramesPerBand returns the number of samples per channel in each sub-band per 10 ms frame.
func (ab *AudioBuffer) FramesPerBand() int {
	return ab.numFrames
}

// SplitChannel returns the sample slice for the given channel and band index.
// band 0 is the lowest sub-band (≤8 kHz), which is where AEC3 and NS operate.
// when Bands() == 1, it returns the full-band slice regardless of the band argument.
func (ab *AudioBuffer) SplitChannel(ch, band int) []float32 {
	if ab.numBands == 1 {
		return ab.data[ch]
	}
	return ab.splitData[ch][band]
}

// CopyFromInterleaved reads interleaved int16 samples into the buffer, converting to FloatS16.
// src must contain exactly TotalSamples() elements; channels are interleaved as [s0ch0, s0ch1, ...].
func (ab *AudioBuffer) CopyFromInterleaved(src []int16) {
	frameSize := int(ab.sampleRateHz) / 100
	for ch := 0; ch < ab.numChannels; ch++ {
		for i := 0; i < frameSize; i++ {
			ab.data[ch][i] = float32(src[i*ab.numChannels+ch])
		}
	}
}

// CopyToInterleaved writes the buffer contents to dst as interleaved int16, clamping to [-32768, 32767].
// dst must have capacity for at least TotalSamples() elements.
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

// CopyFromFloat copies de-interleaved FloatS16 samples from src into the buffer.
// src[ch] must contain at least FrameSize() elements; extra channels in src are ignored.
func (ab *AudioBuffer) CopyFromFloat(src [][]float32) {
	for ch := 0; ch < ab.numChannels && ch < len(src); ch++ {
		copy(ab.data[ch], src[ch])
	}
}

// CopyToFloat copies de-interleaved FloatS16 samples from the buffer into dst.
// dst[ch] must have capacity for at least FrameSize() elements; extra channels in dst are ignored.
func (ab *AudioBuffer) CopyToFloat(dst [][]float32) {
	for ch := 0; ch < ab.numChannels && ch < len(dst); ch++ {
		copy(dst[ch], ab.data[ch])
	}
}

// Clear zeros all full-band and sub-band sample data in the buffer.
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

// SplitIntoFrequencyBands runs the analysis filter bank, populating the per-band sub-buffers
// from the full-band data. must be called before accessing SplitChannel for band > 0.
func (ab *AudioBuffer) SplitIntoFrequencyBands() {
	if ab.splittingFilter != nil && ab.splitData != nil {
		ab.splittingFilter.Analysis(ab.data, ab.splitData)
	}
}

// MergeFrequencyBands runs the synthesis filter bank, reconstructing the full-band signal
// from the per-band sub-buffers. call after modifying sub-band data via SplitChannel.
func (ab *AudioBuffer) MergeFrequencyBands() {
	if ab.splittingFilter != nil && ab.splitData != nil {
		ab.splittingFilter.Synthesis(ab.splitData, ab.data)
	}
}
