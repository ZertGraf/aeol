package rnn_vad

import "math"

const (
	sampleRate24kHz      = 24000
	frameSize10ms24kHz   = 240 // sampleRate24kHz / 100
	frameSize20ms24kHz   = 480 // frameSize10ms24kHz * 2
	minPitch24kHz        = 30  // sampleRate24kHz / 800
	maxPitch24kHz        = 384 // sampleRate24kHz / 62.5 = int(384)
	bufSize24kHz         = 864 // maxPitch24kHz + frameSize20ms24kHz
	initialMinPitch24kHz = 90  // 3 * minPitch24kHz
	initialNumLags24kHz  = 294 // maxPitch24kHz - initialMinPitch24kHz
	refineNumLags24kHz   = 385 // maxPitch24kHz + 1

	sampleRate12kHz      = 12000
	frameSize10ms12kHz   = 120
	frameSize20ms12kHz   = 240
	bufSize12kHz         = 432 // bufSize24kHz / 2
	initialMinPitch12kHz = 45  // initialMinPitch24kHz / 2
	maxPitch12kHz        = 192 // maxPitch24kHz / 2
	numLags12kHz         = 147 // maxPitch12kHz - initialMinPitch12kHz

	minPitch48kHz = 60  // minPitch24kHz * 2
	maxPitch48kHz = 768 // maxPitch24kHz * 2

	numLpcCoefficients  = 5
	featureVectorSize   = 42
	numBands            = 22
	numLowerBands       = 6
	cepstralHistorySize = 8
)

// computeAndPostProcessLpcCoefficients computes 5 LPC coefficients for the given frame x.
// the result is written into lpcCoeffs which must have length >= numLpcCoefficients.
func computeAndPostProcessLpcCoefficients(x []float32, lpcCoeffs []float32) {
	const order = 4

	var autoCorr [order + 1]float32
	for lag := 0; lag <= order; lag++ {
		autoCorr[lag] = innerProduct(x[:len(x)-lag], x[lag:])
	}

	if autoCorr[0] == 0 {
		for i := range lpcCoeffs[:numLpcCoefficients] {
			lpcCoeffs[i] = 0
		}
		return
	}

	autoCorr[0] *= 1.0001
	autoCorr[1] -= autoCorr[1] * 0.000064
	autoCorr[2] -= autoCorr[2] * 0.000256
	autoCorr[3] -= autoCorr[3] * 0.000576
	autoCorr[4] -= autoCorr[4] * 0.001024

	var lpcPre [order]float32
	errVal := autoCorr[0]

	for i := 0; i < order; i++ {
		var reflCoeff float32
		for j := 0; j < i; j++ {
			reflCoeff += lpcPre[j] * autoCorr[i-j]
		}
		reflCoeff += autoCorr[i+1]

		if absF32(errVal) < 1e-6 {
			errVal = float32(math.Copysign(1e-6, float64(errVal)))
		}
		reflCoeff /= -errVal

		lpcPre[i] = reflCoeff
		for j := 0; j < (i+1)>>1; j++ {
			tmp1 := lpcPre[j]
			tmp2 := lpcPre[i-1-j]
			lpcPre[j] = tmp1 + reflCoeff*tmp2
			lpcPre[i-1-j] = tmp2 + reflCoeff*tmp1
		}

		errVal -= reflCoeff * reflCoeff * errVal
		if errVal < 0.001*autoCorr[0] {
			break
		}
	}

	lpcPre[0] *= 0.9
	lpcPre[1] *= 0.81
	lpcPre[2] *= 0.729
	lpcPre[3] *= 0.6561

	lpcCoeffs[0] = lpcPre[0] + 0.8
	lpcCoeffs[1] = lpcPre[1] + 0.8*lpcPre[0]
	lpcCoeffs[2] = lpcPre[2] + 0.8*lpcPre[1]
	lpcCoeffs[3] = lpcPre[3] + 0.8*lpcPre[2]
	lpcCoeffs[4] = 0.8 * lpcPre[3]
}

// computeLpResidual applies the LPC analysis filter to x and writes the LP residual
// into y. x and y may alias (in-place operation is safe).
func computeLpResidual(lpcCoeffs [numLpcCoefficients]float32, x, y []float32) {
	n := len(x)
	if n == 0 {
		return
	}

	y[0] = x[0]

	for i := 1; i < numLpcCoefficients && i < n; i++ {
		sum := x[i]
		for j := 0; j < i; j++ {
			sum += lpcCoeffs[j] * x[i-1-j]
		}
		y[i] = sum
	}

	for i := numLpcCoefficients; i < n; i++ {
		sum := x[i]
		sum += lpcCoeffs[0]*x[i-1] +
			lpcCoeffs[1]*x[i-2] +
			lpcCoeffs[2]*x[i-3] +
			lpcCoeffs[3]*x[i-4] +
			lpcCoeffs[4]*x[i-5]
		y[i] = sum
	}
}

// innerProduct computes the dot product of two equal-length slices.
func innerProduct(a, b []float32) float32 {
	var sum float32
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func absF32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
