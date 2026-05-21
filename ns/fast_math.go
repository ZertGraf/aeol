package ns

import (
	"math"
)

func fastLog(x float32) float32 {
	if x <= 0 {
		return -100
	}
	bits := math.Float32bits(x)
	exponent := float32(int32(bits>>23)-127) + float32(bits&0x7FFFFF)/float32(0x800000)
	return exponent * 0.6931471805599453
}

func fastExp(x float32) float32 {
	if x < -87 {
		return 0
	}
	if x > 88 {
		return math.MaxFloat32
	}
	return float32(math.Exp(float64(x)))
}

func fastPow(base, exponent float32) float32 {
	return fastExp(exponent * fastLog(base))
}
