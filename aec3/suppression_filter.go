package aec3

import (
	"sonora/fft"
)

type SuppressionFilter struct {
	fftProcessor     *fft.OouraFFT
	eOutputOld       [FFTLengthBy2]float32
	scratchNoiseGain [FFTSizeBy2Plus1]float32
	scratchEFreq     FftData
	scratchBuf       [FFTSize]float32
}

func NewSuppressionFilter() *SuppressionFilter {
	return &SuppressionFilter{
		fftProcessor: fft.NewOouraFFT(),
	}
}

func (sf *SuppressionFilter) ApplyGain(
	comfortNoise *FftData,
	suppressionGain [FFTSizeBy2Plus1]float32,
	eFft *FftData,
	output []float32,
) {
	noiseGain := sf.scratchNoiseGain[:]
	for k := 0; k < FFTSizeBy2Plus1; k++ {
		sg := suppressionGain[k]
		v := 1.0 - sg*sg
		if v < 0 {
			v = 0
		}
		noiseGain[k] = v
	}
	elementwiseSqrt(noiseGain)

	sf.scratchEFreq.CopyFrom(eFft)

	for k := 0; k < FFTSizeBy2Plus1; k++ {
		eReal := sf.scratchEFreq.Re[k] * suppressionGain[k]
		eImag := sf.scratchEFreq.Im[k] * suppressionGain[k]

		if comfortNoise != nil {
			eReal += noiseGain[k] * comfortNoise.Re[k]
			eImag += noiseGain[k] * comfortNoise.Im[k]
		}

		sf.scratchEFreq.Re[k] = eReal
		sf.scratchEFreq.Im[k] = eImag
	}

	sf.fftProcessor.InverseSplit(sf.scratchEFreq.Re[:], sf.scratchEFreq.Im[:], sf.scratchBuf[:])

	for i := 0; i < FFTLengthBy2; i++ {
		v := sf.eOutputOld[i]*sqrtHanning[FFTLengthBy2+i] +
			sf.scratchBuf[i]*sqrtHanning[i]
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		output[i] = v
	}

	copy(sf.eOutputOld[:], sf.scratchBuf[FFTLengthBy2:])
}

func (sf *SuppressionFilter) Reset() {
	clear(sf.eOutputOld[:])
}
