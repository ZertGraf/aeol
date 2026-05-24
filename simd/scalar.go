package simd

import "math"

type scalarBackend struct{}

func (s *scalarBackend) Type() BackendType { return Scalar }

func (s *scalarBackend) DotProduct(a, b []float32) float32 {
	n := min(len(a), len(b))
	var sum float32
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func (s *scalarBackend) DualDotProduct(input, k1, k2 []float32) (float32, float32) {
	n := min(len(input), min(len(k1), len(k2)))
	var sum1, sum2 float32
	for i := 0; i < n; i++ {
		sum1 += input[i] * k1[i]
		sum2 += input[i] * k2[i]
	}
	return sum1, sum2
}

func (s *scalarBackend) Sum(x []float32) float32 {
	var sum float32
	for _, v := range x {
		sum += v
	}
	return sum
}

func (s *scalarBackend) MultiplyAccumulate(acc, a, b []float32) {
	n := min(len(acc), min(len(a), len(b)))
	for i := 0; i < n; i++ {
		acc[i] += a[i] * b[i]
	}
}

func (s *scalarBackend) ElementwiseSqrt(x []float32) {
	for i := range x {
		x[i] = float32(math.Sqrt(float64(x[i])))
	}
}

func (s *scalarBackend) ElementwiseMultiply(x, y, z []float32) {
	n := min(len(x), min(len(y), len(z)))
	for i := 0; i < n; i++ {
		z[i] = x[i] * y[i]
	}
}

func (s *scalarBackend) ElementwiseAccumulate(x, z []float32) {
	n := min(len(x), len(z))
	for i := 0; i < n; i++ {
		z[i] += x[i]
	}
}

func (s *scalarBackend) ElementwiseMin(a, b, out []float32) {
	n := min(len(a), min(len(b), len(out)))
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			out[i] = a[i]
		} else {
			out[i] = b[i]
		}
	}
}

func (s *scalarBackend) ElementwiseMax(a, b, out []float32) {
	n := min(len(a), min(len(b), len(out)))
	for i := 0; i < n; i++ {
		if a[i] > b[i] {
			out[i] = a[i]
		} else {
			out[i] = b[i]
		}
	}
}

func (s *scalarBackend) PowerSpectrum(re, im, out []float32) {
	n := min(len(re), min(len(im), len(out)))
	for i := 0; i < n; i++ {
		out[i] = re[i]*re[i] + im[i]*im[i]
	}
}

func (s *scalarBackend) ComplexMultiplyAccumulate(reA, imA, reB, imB, reOut, imOut []float32) {
	n := min(len(reA), min(len(imA), min(len(reB), min(len(imB), min(len(reOut), len(imOut))))))
	for i := 0; i < n; i++ {
		reOut[i] += reA[i]*reB[i] + imA[i]*imB[i]
		imOut[i] += -reA[i]*imB[i] + imA[i]*reB[i]
	}
}

func (s *scalarBackend) ComplexMultiplyAccumulateStandard(reA, imA, reB, imB, reOut, imOut []float32) {
	n := min(len(reA), min(len(imA), min(len(reB), min(len(imB), min(len(reOut), len(imOut))))))
	for i := 0; i < n; i++ {
		reOut[i] += reA[i]*reB[i] - imA[i]*imB[i]
		imOut[i] += reA[i]*imB[i] + imA[i]*reB[i]
	}
}

func (s *scalarBackend) ScaledComplexMultiplyAccumulate(reA, imA, reB, imB, reOut, imOut []float32, scale float32) {
	n := min(len(reA), min(len(imA), min(len(reB), min(len(imB), min(len(reOut), len(imOut))))))
	for i := 0; i < n; i++ {
		reOut[i] += scale * (reA[i]*reB[i] + imA[i]*imB[i])
		imOut[i] += scale * (-reA[i]*imB[i] + imA[i]*reB[i])
	}
}

func (s *scalarBackend) ConvolveSinc(input []float32, k1, k2 []float64, factor float64) float32 {
	n := min(len(input), min(len(k1), len(k2)))
	var sum1, sum2 float64
	for i := 0; i < n; i++ {
		sum1 += float64(input[i]) * k1[i]
		sum2 += float64(input[i]) * k2[i]
	}
	return float32(sum1 + (sum2-sum1)*factor)
}
