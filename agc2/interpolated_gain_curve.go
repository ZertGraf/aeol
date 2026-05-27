package agc2

// interpolatedGainCurve provides a piecewise-linear under-approximation of the
// limiter gain curve to avoid saturation.
type interpolatedGainCurve struct{}

func newInterpolatedGainCurve() *interpolatedGainCurve {
	return &interpolatedGainCurve{}
}

const maxInputLevelLinear = 36766.3

var interpolatedGainCurveApproxX = [...]float32{
	30057.297, 30148.986, 30240.676, 30424.053, 30607.43, 30790.807, 30974.184, 31157.56,
	31340.94, 31524.316, 31707.693, 31891.07, 32074.447, 32257.824, 32441.201, 32624.58,
	32807.957, 32991.332, 33174.71, 33358.09, 33541.465, 33724.844, 33819.535, 34009.54,
	34200.06, 34389.816, 34674.49, 35054.375, 35434.863, 35814.816, 36195.168, 36575.03,
}

var interpolatedGainCurveApproxM = [...]float32{
	-3.5152357e-07,
	-1.0502516e-06,
	-2.0852137e-06,
	-3.4430047e-06,
	-4.7738495e-06,
	-6.077376e-06,
	-7.353258e-06,
	-8.60122e-06,
	-9.821013e-06,
	-1.1012434e-05,
	-1.2175326e-05,
	-1.3309569e-05,
	-1.4415075e-05,
	-1.5491793e-05,
	-1.6539707e-05,
	-1.7558828e-05,
	-1.8549184e-05,
	-1.9510868e-05,
	-2.044398e-05,
	-2.1348627e-05,
	-2.222497e-05,
	-2.2653747e-05,
	-2.242571e-05,
	-2.220122e-05,
	-2.198021e-05,
	-2.1762602e-05,
	-2.1337317e-05,
	-2.092482e-05,
	-2.0524596e-05,
	-2.0136154e-05,
	-1.975903e-05,
	-1.9392779e-05,
}

var interpolatedGainCurveApproxQ = [...]float32{
	1.0105659,
	1.0316318,
	1.0629297,
	1.1042392,
	1.144973,
	1.1851096,
	1.224629,
	1.2635125,
	1.301742,
	1.3393006,
	1.3761733,
	1.4123455,
	1.447804,
	1.4825366,
	1.5165322,
	1.5497806,
	1.5822722,
	1.6139994,
	1.644955,
	1.6751324,
	1.7045262,
	1.7189866,
	1.7112745,
	1.7036397,
	1.6960812,
	1.6885977,
	1.6738511,
	1.6593913,
	1.6452094,
	1.6312975,
	1.6176474,
	1.6042517,
}

func (c *interpolatedGainCurve) LookUpGainToApply(inputLevel float32) float32 {
	if inputLevel <= 0 {
		return 1.0
	}
	if inputLevel <= interpolatedGainCurveApproxX[0] {
		return 1.0
	}
	if inputLevel >= maxInputLevelLinear {
		return float32(maxAbsFloatS16) / inputLevel
	}

	idx := interpolatedGainCurveIndex(inputLevel)
	gain := interpolatedGainCurveApproxM[idx]*inputLevel + interpolatedGainCurveApproxQ[idx]
	if gain < 0 {
		return 0
	}
	return gain
}

func interpolatedGainCurveIndex(inputLevel float32) int {
	lo := 0
	hi := len(interpolatedGainCurveApproxX) - 1
	for lo <= hi {
		mid := (lo + hi) / 2
		if interpolatedGainCurveApproxX[mid] <= inputLevel {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if hi < 0 {
		return 0
	}
	if hi >= len(interpolatedGainCurveApproxM) {
		return len(interpolatedGainCurveApproxM) - 1
	}
	return hi
}
