// Package hpf implements a high-pass filter stage for the aeol pipeline.
// The filter operates on FloatS16 data (float32 values in [-32768, 32767]).
package hpf

import "aeol/dsp"

// biquad coefficients keyed by sample rate.
// derived from WebRTC's high_pass_filter.cc (M145).
var coeffsByRate = map[uint32][]dsp.BiQuadCoefficients{
	16000: {
		{
			B: [3]float32{0.9770, -1.9540, 0.9770},
			A: [2]float32{-1.9534, 0.9546},
		},
	},
	32000: {
		{
			B: [3]float32{0.9883, -1.9767, 0.9883},
			A: [2]float32{-1.9764, 0.9770},
		},
	},
	48000: {
		{
			B: [3]float32{0.9922, -1.9844, 0.9922},
			A: [2]float32{-1.9843, 0.9846},
		},
	},
}

// Filter is a single-channel high-pass filter backed by a cascaded biquad.
// Each channel in AudioProcessing gets its own Filter instance.
type Filter struct {
	biquad *dsp.CascadedBiQuadFilter
}

// New returns a Filter for the given sample rate.
// If the sample rate is not one of 16000, 32000, 48000, the 48 kHz
// coefficients are used as a safe fallback.
func New(sampleRateHz uint32) *Filter {
	coeffs, ok := coeffsByRate[sampleRateHz]
	if !ok {
		coeffs = coeffsByRate[48000]
	}
	return &Filter{biquad: dsp.NewCascadedBiQuadFilter(coeffs)}
}

// Process applies the high-pass filter in-place to a single channel of
// FloatS16 samples.
func (f *Filter) Process(samples []float32) {
	f.biquad.ProcessInPlace(samples)
}

// Reset clears the filter state.
func (f *Filter) Reset() {
	f.biquad.Reset()
}
