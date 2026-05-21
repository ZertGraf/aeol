package simd

import "math"

type avx2Backend struct{}

func (a *avx2Backend) Type() BackendType { return AVX2 }

func (a *avx2Backend) DotProduct(x, y []float32) float32 {
	n := min(len(x), len(y))
	var sum float32
	i := 0
	for ; i+7 < n; i += 8 {
		sum += x[i]*y[i] + x[i+1]*y[i+1] + x[i+2]*y[i+2] + x[i+3]*y[i+3] +
			x[i+4]*y[i+4] + x[i+5]*y[i+5] + x[i+6]*y[i+6] + x[i+7]*y[i+7]
	}
	for ; i < n; i++ {
		sum += x[i] * y[i]
	}
	return sum
}

func (a *avx2Backend) DualDotProduct(input, k1, k2 []float32) (float32, float32) {
	n := min(len(input), min(len(k1), len(k2)))
	var sum1, sum2 float32
	i := 0
	for ; i+7 < n; i += 8 {
		for j := 0; j < 8; j++ {
			sum1 += input[i+j] * k1[i+j]
			sum2 += input[i+j] * k2[i+j]
		}
	}
	for ; i < n; i++ {
		sum1 += input[i] * k1[i]
		sum2 += input[i] * k2[i]
	}
	return sum1, sum2
}

func (a *avx2Backend) Sum(x []float32) float32 {
	var sum float32
	i := 0
	for ; i+7 < len(x); i += 8 {
		sum += x[i] + x[i+1] + x[i+2] + x[i+3] + x[i+4] + x[i+5] + x[i+6] + x[i+7]
	}
	for ; i < len(x); i++ {
		sum += x[i]
	}
	return sum
}

func (a *avx2Backend) MultiplyAccumulate(acc, x, y []float32) {
	n := min(len(acc), min(len(x), len(y)))
	i := 0
	for ; i+7 < n; i += 8 {
		acc[i] += x[i] * y[i]
		acc[i+1] += x[i+1] * y[i+1]
		acc[i+2] += x[i+2] * y[i+2]
		acc[i+3] += x[i+3] * y[i+3]
		acc[i+4] += x[i+4] * y[i+4]
		acc[i+5] += x[i+5] * y[i+5]
		acc[i+6] += x[i+6] * y[i+6]
		acc[i+7] += x[i+7] * y[i+7]
	}
	for ; i < n; i++ {
		acc[i] += x[i] * y[i]
	}
}

func (a *avx2Backend) ElementwiseSqrt(x []float32) {
	for i := range x {
		x[i] = float32(math.Sqrt(float64(x[i])))
	}
}

func (a *avx2Backend) ElementwiseMultiply(x, y, z []float32) {
	n := min(len(x), min(len(y), len(z)))
	i := 0
	for ; i+7 < n; i += 8 {
		z[i] = x[i] * y[i]
		z[i+1] = x[i+1] * y[i+1]
		z[i+2] = x[i+2] * y[i+2]
		z[i+3] = x[i+3] * y[i+3]
		z[i+4] = x[i+4] * y[i+4]
		z[i+5] = x[i+5] * y[i+5]
		z[i+6] = x[i+6] * y[i+6]
		z[i+7] = x[i+7] * y[i+7]
	}
	for ; i < n; i++ {
		z[i] = x[i] * y[i]
	}
}

func (a *avx2Backend) ElementwiseAccumulate(x, z []float32) {
	n := min(len(x), len(z))
	i := 0
	for ; i+7 < n; i += 8 {
		z[i] += x[i]
		z[i+1] += x[i+1]
		z[i+2] += x[i+2]
		z[i+3] += x[i+3]
		z[i+4] += x[i+4]
		z[i+5] += x[i+5]
		z[i+6] += x[i+6]
		z[i+7] += x[i+7]
	}
	for ; i < n; i++ {
		z[i] += x[i]
	}
}

func (a *avx2Backend) ElementwiseMin(x, y, out []float32) {
	n := min(len(x), min(len(y), len(out)))
	for i := 0; i < n; i++ {
		if x[i] < y[i] {
			out[i] = x[i]
		} else {
			out[i] = y[i]
		}
	}
}

func (a *avx2Backend) ElementwiseMax(x, y, out []float32) {
	n := min(len(x), min(len(y), len(out)))
	for i := 0; i < n; i++ {
		if x[i] > y[i] {
			out[i] = x[i]
		} else {
			out[i] = y[i]
		}
	}
}

func (a *avx2Backend) PowerSpectrum(re, im, out []float32) {
	n := min(len(re), min(len(im), len(out)))
	i := 0
	for ; i+7 < n; i += 8 {
		out[i] = re[i]*re[i] + im[i]*im[i]
		out[i+1] = re[i+1]*re[i+1] + im[i+1]*im[i+1]
		out[i+2] = re[i+2]*re[i+2] + im[i+2]*im[i+2]
		out[i+3] = re[i+3]*re[i+3] + im[i+3]*im[i+3]
		out[i+4] = re[i+4]*re[i+4] + im[i+4]*im[i+4]
		out[i+5] = re[i+5]*re[i+5] + im[i+5]*im[i+5]
		out[i+6] = re[i+6]*re[i+6] + im[i+6]*im[i+6]
		out[i+7] = re[i+7]*re[i+7] + im[i+7]*im[i+7]
	}
	for ; i < n; i++ {
		out[i] = re[i]*re[i] + im[i]*im[i]
	}
}

func (a *avx2Backend) ComplexMultiplyAccumulate(reA, imA, reB, imB, reOut, imOut []float32) {
	n := min(len(reA), min(len(imA), min(len(reB), min(len(imB), min(len(reOut), len(imOut))))))
	for i := 0; i < n; i++ {
		reOut[i] += reA[i]*reB[i] + imA[i]*imB[i]
		imOut[i] += -reA[i]*imB[i] + imA[i]*reB[i]
	}
}

func (a *avx2Backend) ComplexMultiplyAccumulateStandard(reA, imA, reB, imB, reOut, imOut []float32) {
	n := min(len(reA), min(len(imA), min(len(reB), min(len(imB), min(len(reOut), len(imOut))))))
	for i := 0; i < n; i++ {
		reOut[i] += reA[i]*reB[i] - imA[i]*imB[i]
		imOut[i] += reA[i]*imB[i] + imA[i]*reB[i]
	}
}

func (a *avx2Backend) ConvolveSinc(input []float32, k1, k2 []float64, factor float64) float32 {
	n := min(len(input), min(len(k1), len(k2)))
	var sum1, sum2 float64
	for i := 0; i < n; i++ {
		v := float64(input[i])
		sum1 += v * k1[i]
		sum2 += v * k2[i]
	}
	return float32(sum1 + (sum2-sum1)*factor)
}
