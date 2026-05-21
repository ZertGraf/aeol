// Package dsp provides shared audio processing utilities including
// channel buffers, biquad filters, and resamplers.
package dsp

import "math"

const (
	MaxS16     = 32767
	MinS16     = -32768
	S16ToFloat = 1.0 / 32768.0
	FloatToS16 = 32768.0
)

func S16ToFloatS16(v int16) float32 {
	return float32(v)
}

func FloatS16ToS16(v float32) int16 {
	v = float32(math.Round(float64(v)))
	if v > MaxS16 {
		return MaxS16
	}
	if v < MinS16 {
		return MinS16
	}
	return int16(v)
}

func S16ToFloatNorm(v int16) float32 {
	return float32(v) * S16ToFloat
}

func FloatNormToS16(v float32) int16 {
	return FloatS16ToS16(v * FloatToS16)
}

func FloatS16ToDbfs(v float32) float32 {
	if v <= 0 {
		return -100
	}
	return float32(20 * math.Log10(float64(v)/FloatToS16))
}

func Deinterleave(interleaved []float32, channels int, out [][]float32) {
	framesPerChannel := len(interleaved) / channels
	for ch := 0; ch < channels; ch++ {
		for i := 0; i < framesPerChannel; i++ {
			out[ch][i] = interleaved[i*channels+ch]
		}
	}
}

func Interleave(planar [][]float32, channels int, out []float32) {
	if channels == 0 || len(planar) == 0 {
		return
	}
	framesPerChannel := len(planar[0])
	for ch := 0; ch < channels; ch++ {
		for i := 0; i < framesPerChannel; i++ {
			out[i*channels+ch] = planar[ch][i]
		}
	}
}

func DeinterleaveS16(interleaved []int16, channels int, out [][]float32) {
	framesPerChannel := len(interleaved) / channels
	for ch := 0; ch < channels; ch++ {
		for i := 0; i < framesPerChannel; i++ {
			out[ch][i] = S16ToFloatNorm(interleaved[i*channels+ch])
		}
	}
}

func InterleaveToS16(planar [][]float32, channels int, out []int16) {
	if channels == 0 || len(planar) == 0 {
		return
	}
	framesPerChannel := len(planar[0])
	for ch := 0; ch < channels; ch++ {
		for i := 0; i < framesPerChannel; i++ {
			out[i*channels+ch] = FloatNormToS16(planar[ch][i])
		}
	}
}

func DownmixToMono(channels [][]float32, mono []float32) {
	nch := len(channels)
	if nch == 0 {
		return
	}
	if nch == 1 {
		copy(mono, channels[0])
		return
	}
	n := len(mono)
	scale := 1.0 / float32(nch)
	for i := 0; i < n; i++ {
		var sum float32
		for ch := 0; ch < nch; ch++ {
			sum += channels[ch][i]
		}
		mono[i] = sum * scale
	}
}

func UpmixFromMono(mono []float32, channels [][]float32) {
	for ch := range channels {
		copy(channels[ch], mono)
	}
}

func RmsLevel(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return float32(math.Sqrt(sum / float64(len(samples))))
}
