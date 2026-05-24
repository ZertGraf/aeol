package aec3

import (
	"aeol/fft"
)

// SuppressionFilter applies frequency-domain Wiener-like gain to the error signal
// and blends in comfort noise, then reconstructs the time-domain output via
// inverse FFT and overlap-add with a sqrt-Hanning window.
type SuppressionFilter struct {
	fftProcessor     fft.FFT
	eOutputOld       [FFTLengthBy2]float32
	scratchNoiseGain [FFTSizeBy2Plus1]float32
	scratchEFreq     FftData
	scratchBuf       [FFTSize]float32
}

// NewSuppressionFilter creates a SuppressionFilter.
// fftFactory is optional; if omitted the default Ooura FFT backend is used.
func NewSuppressionFilter(fftFactory ...fft.Factory) *SuppressionFilter {
	factory := fft.DefaultFactory
	if len(fftFactory) > 0 && fftFactory[0] != nil {
		factory = fftFactory[0]
	}
	return &SuppressionFilter{
		fftProcessor: factory(FFTSize),
	}
}

// ApplyGain multiplies each frequency bin of eFft by suppressionGain, adds comfort noise
// scaled by the complementary gain sqrt(1 - g^2), then reconstructs BlockSize FloatS16
// samples into output via IFFT and overlap-add.
// comfortNoise may be nil to skip noise injection.
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

	fft.InverseSplit(sf.fftProcessor, sf.scratchEFreq.Re[:], sf.scratchEFreq.Im[:], sf.scratchBuf[:])

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

// Reset clears the overlap-add tail buffer, eliminating any residual output from prior frames.
func (sf *SuppressionFilter) Reset() {
	clear(sf.eOutputOld[:])
}
