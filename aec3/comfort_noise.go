package aec3

import (
	"math"
)

const (
	cngSeedMask          = 0x7FFFFFFF
	cngImIndexMask       = 31
	cngSmoothingAlpha    = 0.1
	cngN2UpdateThreshold = 50
	cngInitialPhaseDur   = 1000
	cngN2TrackUp         = 1.0002
	cngN2InitialValue    = 1e6
	cngN2InitialBlend    = 0.001
	defaultNoiseFloorDb  = -50
)

var sqrt2Sin = [32]float32{
	0.0000000, 0.2758994, 0.5411961, 0.7856950, 1.0000000, 1.1758756, 1.3065630, 1.3870398,
	1.4142135, 1.3870398, 1.3065630, 1.1758756, 1.0000000, 0.7856950, 0.5411961, 0.2758994,
	0.0000000, -0.2758994, -0.5411961, -0.7856950, -1.0000000, -1.1758756, -1.3065630, -1.3870398,
	-1.4142135, -1.3870398, -1.3065630, -1.1758756, -1.0000000, -0.7856950, -0.5411961, -0.2758994,
}

// ComfortNoiseGenerator estimates background noise power per frequency bin and
// synthesizes spectrally shaped pseudo-random noise for comfort noise injection.
type ComfortNoiseGenerator struct {
	seed       uint32
	noiseFloor float32
	n2         [FFTSizeBy2Plus1]float32
	n2Initial  [FFTSizeBy2Plus1]float32
	y2Smoothed [FFTSizeBy2Plus1]float32
	n2Counter  int
	useInitial bool
	scratchN   [FFTSizeBy2Plus1]float32
}

// NewComfortNoiseGenerator creates a ComfortNoiseGenerator with a default noise floor of -50 dBFS.
func NewComfortNoiseGenerator() *ComfortNoiseGenerator {
	cng := &ComfortNoiseGenerator{
		seed:       42,
		noiseFloor: getNoiseFloorFactor(defaultNoiseFloorDb),
		useInitial: true,
	}
	for k := range cng.n2 {
		cng.n2[k] = cngN2InitialValue
		cng.n2Initial[k] = 0
	}
	return cng
}

func getNoiseFloorFactor(noiseFloorDbfs float32) float32 {
	const dbfsNormalization = 90.30899
	return 64 * float32(math.Pow(10, float64((dbfsNormalization+noiseFloorDbfs)*0.1)))
}

// Compute updates the noise estimate from captureSpectrum and writes shaped comfort noise
// into out as a half-spectrum (FFTSizeBy2Plus1 complex bins).
// set saturated to true to freeze the noise tracker during clipping events.
func (cng *ComfortNoiseGenerator) Compute(saturated bool, captureSpectrum [FFTSizeBy2Plus1]float32, out *FftData) {
	if !saturated {
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			cng.y2Smoothed[k] += cngSmoothingAlpha * (captureSpectrum[k] - cng.y2Smoothed[k])
		}

		cng.n2Counter++
		if cng.n2Counter > cngN2UpdateThreshold {
			for k := 0; k < FFTSizeBy2Plus1; k++ {
				a := cng.n2[k]
				b := cng.y2Smoothed[k]
				if b < a {
					cng.n2[k] = (0.9*b + 0.1*a) * cngN2TrackUp
				} else {
					cng.n2[k] = a * cngN2TrackUp
				}
			}
		}

		if cng.useInitial {
			if cng.n2Counter >= cngInitialPhaseDur {
				cng.useInitial = false
			} else {
				for k := 0; k < FFTSizeBy2Plus1; k++ {
					a := cng.n2[k]
					b := cng.n2Initial[k]
					if a > b {
						cng.n2Initial[k] = b + cngN2InitialBlend*(a-b)
					} else {
						cng.n2Initial[k] = a
					}
				}
			}
		}

		for k := 0; k < FFTSizeBy2Plus1; k++ {
			if cng.n2[k] < cng.noiseFloor {
				cng.n2[k] = cng.noiseFloor
			}
			if cng.useInitial && cng.n2Initial[k] < cng.noiseFloor {
				cng.n2Initial[k] = cng.noiseFloor
			}
		}
	}

	src := &cng.n2
	if cng.useInitial {
		src = &cng.n2Initial
	}

	copy(cng.scratchN[:], src[:])
	elementwiseSqrt(cng.scratchN[:])

	out.Re[0] = 0
	out.Im[0] = 0
	out.Re[FFTLengthBy2] = 0
	out.Im[FFTLengthBy2] = 0

	for k := 1; k < FFTLengthBy2; k++ {
		cng.seed = (cng.seed*69069 + 1) & cngSeedMask
		reIdx := cng.seed >> 26
		imIdx := (reIdx + 8) & cngImIndexMask

		out.Re[k] = cng.scratchN[k] * sqrt2Sin[reIdx]
		out.Im[k] = cng.scratchN[k] * sqrt2Sin[imIdx]
	}
}

// NoiseSpectrum returns a pointer to the current per-bin noise power estimate array.
// the array has FFTSizeBy2Plus1 (65) elements in FloatS16^2 units.
func (cng *ComfortNoiseGenerator) NoiseSpectrum() *[FFTSizeBy2Plus1]float32 {
	return &cng.n2
}
