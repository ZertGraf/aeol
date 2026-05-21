package dsp

import "math"

const (
	NumBands      = 3
	FullBandSize  = 480
	SplitBandSize = FullBandSize / NumBands
)

const (
	sparsity          = 4
	strideLog2        = 2
	stride            = 1 << strideLog2
	numZeroFilters    = 2
	filterSize        = 4
	memorySize        = filterSize*stride - 1
	numNonZeroFilters = sparsity*NumBands - numZeroFilters
	subSampling       = NumBands
	zeroFilterIndex1  = 3
	zeroFilterIndex2  = 9
)

var filterCoeffs = [numNonZeroFilters][filterSize]float32{
	{-0.00047749, -0.00496888, 0.16547118, 0.00425496},
	{-0.00173287, -0.01585778, 0.14989004, 0.00994113},
	{-0.00304815, -0.02536082, 0.12154542, 0.01157993},
	{-0.00346946, -0.02587886, 0.04760441, 0.00607594},
	{-0.00154717, -0.01136076, 0.01387458, 0.00186353},
	{0.00186353, 0.01387458, -0.01136076, -0.00154717},
	{0.00607594, 0.04760441, -0.02587886, -0.00346946},
	{0.00983212, 0.08543175, -0.02982767, -0.00383509},
	{0.00994113, 0.14989004, -0.01585778, -0.00173287},
	{0.00425496, 0.16547118, -0.00496888, -0.00047749},
}

var dctModulation = [numNonZeroFilters][NumBands]float32{
	{2.0, 2.0, 2.0},
	{float32(math.Sqrt(3)), 0.0, float32(-math.Sqrt(3))},
	{1.0, -2.0, 1.0},
	{-1.0, 2.0, -1.0},
	{float32(-math.Sqrt(3)), 0.0, float32(math.Sqrt(3))},
	{-2.0, -2.0, -2.0},
	{float32(-math.Sqrt(3)), 0.0, float32(math.Sqrt(3))},
	{-1.0, 2.0, -1.0},
	{1.0, -2.0, 1.0},
	{float32(math.Sqrt(3)), 0.0, float32(-math.Sqrt(3))},
}

func filterCore(filter []float32, input []float32, inShift int, output []float32, state []float32) {
	for i := range output {
		output[i] = 0.0
	}
	f0, f1, f2, f3 := filter[0], filter[1], filter[2], filter[3]

	for k := 0; k < inShift; k++ {
		j := memorySize + k - inShift
		output[k] = state[j]*f0 + state[j-stride]*f1 + state[j-2*stride]*f2 + state[j-3*stride]*f3
	}

	for k := inShift; k < filterSize*stride; k++ {
		shift := k - inShift
		loopLimit := 1 + (shift >> strideLog2)
		if loopLimit > filterSize {
			loopLimit = filterSize
		}

		for i := 0; i < loopLimit; i++ {
			output[k] += input[shift-i*stride] * filter[i]
		}

		jBase := memorySize + shift - loopLimit*stride
		for i := loopLimit; i < filterSize; i++ {
			output[k] += state[jBase-(i-loopLimit)*stride] * filter[i]
		}
	}

	for k := filterSize * stride; k < SplitBandSize; k++ {
		base := k - inShift
		output[k] = input[base]*f0 + input[base-stride]*f1 + input[base-2*stride]*f2 + input[base-3*stride]*f3
	}

	copy(state, input[SplitBandSize-memorySize:])
}

type ThreeBandFilterBank struct {
	stateAnalysis  [numNonZeroFilters][memorySize]float32
	stateSynthesis [numNonZeroFilters][memorySize]float32
}

func NewThreeBandFilterBank() *ThreeBandFilterBank {
	return &ThreeBandFilterBank{}
}

func (fb *ThreeBandFilterBank) Analysis(input []float32, output [][]float32) {
	for band := 0; band < NumBands; band++ {
		for i := range output[band] {
			output[band][i] = 0.0
		}
	}

	var inSubsampled [SplitBandSize]float32
	var outSubsampled [SplitBandSize]float32

	for downsamplingIndex := 0; downsamplingIndex < subSampling; downsamplingIndex++ {
		for k := 0; k < SplitBandSize; k++ {
			inSubsampled[k] = input[(subSampling-1)-downsamplingIndex+subSampling*k]
		}

		for inShift := 0; inShift < stride; inShift++ {
			index := downsamplingIndex + inShift*subSampling
			if index == zeroFilterIndex1 || index == zeroFilterIndex2 {
				continue
			}
			filterIndex := index
			if index > zeroFilterIndex2 {
				filterIndex = index - 2
			} else if index > zeroFilterIndex1 {
				filterIndex = index - 1
			}

			filter := filterCoeffs[filterIndex][:]
			dctMod := dctModulation[filterIndex][:]

			filterCore(filter, inSubsampled[:], inShift, outSubsampled[:], fb.stateAnalysis[filterIndex][:])

			for band := 0; band < NumBands; band++ {
				modVal := dctMod[band]
				for n := 0; n < SplitBandSize; n++ {
					output[band][n] += modVal * outSubsampled[n]
				}
			}
		}
	}
}

func (fb *ThreeBandFilterBank) Synthesis(input [][]float32, output []float32) {
	for i := range output {
		output[i] = 0.0
	}

	var inSubsampled [SplitBandSize]float32
	var outSubsampled [SplitBandSize]float32

	for upsamplingIndex := 0; upsamplingIndex < subSampling; upsamplingIndex++ {
		for inShift := 0; inShift < stride; inShift++ {
			index := upsamplingIndex + inShift*subSampling
			if index == zeroFilterIndex1 || index == zeroFilterIndex2 {
				continue
			}
			filterIndex := index
			if index > zeroFilterIndex2 {
				filterIndex = index - 2
			} else if index > zeroFilterIndex1 {
				filterIndex = index - 1
			}

			filter := filterCoeffs[filterIndex][:]
			dctMod := dctModulation[filterIndex][:]

			for n := 0; n < SplitBandSize; n++ {
				inSubsampled[n] = 0.0
			}
			for band := 0; band < NumBands; band++ {
				modVal := dctMod[band]
				for n := 0; n < SplitBandSize; n++ {
					inSubsampled[n] += modVal * input[band][n]
				}
			}

			filterCore(filter, inSubsampled[:], inShift, outSubsampled[:], fb.stateSynthesis[filterIndex][:])

			upsamplingScaling := float32(subSampling)
			for k := 0; k < SplitBandSize; k++ {
				output[upsamplingIndex+subSampling*k] += upsamplingScaling * outSubsampled[k]
			}
		}
	}
}
