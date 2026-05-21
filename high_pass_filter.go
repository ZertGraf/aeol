package sonora

import "sonora/dsp"

var hpfCoeffs = map[uint32][]dsp.BiQuadCoefficients{
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

type HighPassFilter struct {
	filters []*dsp.CascadedBiQuadFilter
}

func NewHighPassFilter(sampleRate uint32, numChannels int) *HighPassFilter {
	coeffs, ok := hpfCoeffs[sampleRate]
	if !ok {
		coeffs = hpfCoeffs[48000]
	}

	filters := make([]*dsp.CascadedBiQuadFilter, numChannels)
	for ch := range filters {
		filters[ch] = dsp.NewCascadedBiQuadFilter(coeffs)
	}

	return &HighPassFilter{filters: filters}
}

func (hpf *HighPassFilter) Process(channels [][]float32) {
	for ch := 0; ch < len(hpf.filters) && ch < len(channels); ch++ {
		hpf.filters[ch].ProcessInPlace(channels[ch])
	}
}

func (hpf *HighPassFilter) Reset() {
	for _, f := range hpf.filters {
		f.Reset()
	}
}
