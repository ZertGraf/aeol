// Package simd provides runtime-selectable SIMD backends for audio DSP operations.
package simd

type BackendType int

const (
	Scalar BackendType = iota
	SSE2
	Unrolled
	NEON
)

func (b BackendType) String() string {
	switch b {
	case SSE2:
		return "SSE2"
	case Unrolled:
		return "Unrolled"
	case NEON:
		return "NEON"
	default:
		return "Scalar"
	}
}

type Backend interface {
	Type() BackendType

	DotProduct(a, b []float32) float32
	DualDotProduct(input, k1, k2 []float32) (float32, float32)
	Sum(x []float32) float32

	MultiplyAccumulate(acc, a, b []float32)
	ElementwiseSqrt(x []float32)
	ElementwiseMultiply(x, y, z []float32)
	ElementwiseAccumulate(x, z []float32)
	ElementwiseMin(a, b, out []float32)
	ElementwiseMax(a, b, out []float32)

	PowerSpectrum(re, im, out []float32)
	ComplexMultiplyAccumulate(reA, imA, reB, imB, reOut, imOut []float32)
	ComplexMultiplyAccumulateStandard(reA, imA, reB, imB, reOut, imOut []float32)

	// ScaledComplexMultiplyAccumulate computes the scaled conjugate-multiply-accumulate:
	//   outRe[k] += scale * (reA[k]*reB[k] + imA[k]*imB[k])
	//   outIm[k] += scale * (-reA[k]*imB[k] + imA[k]*reB[k])
	// used for NLMS adaptive filter coefficient updates.
	ScaledComplexMultiplyAccumulate(reA, imA, reB, imB, reOut, imOut []float32, scale float32)

	ConvolveSinc(input []float32, k1, k2 []float64, factor float64) float32
}

var defaultBackend Backend

func init() {
	defaultBackend = detectBackend()
}

func Default() Backend {
	return defaultBackend
}

func Available() []BackendType {
	return availableBackends()
}
