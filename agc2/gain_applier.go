package agc2

type gainApplier struct {
	currentGainLinear float32
	targetGainLinear  float32
}

func newGainApplier(initialGainDb float32) *gainApplier {
	linear := dbToLinear(initialGainDb)
	return &gainApplier{
		currentGainLinear: linear,
		targetGainLinear:  linear,
	}
}

func (ga *gainApplier) SetGainDb(gainDb float32) {
	ga.targetGainLinear = dbToLinear(gainDb)
}

func (ga *gainApplier) Apply(samples []float32) {
	n := len(samples)
	if n == 0 {
		return
	}

	if ga.currentGainLinear == ga.targetGainLinear {
		if ga.currentGainLinear == 1.0 {
			return
		}
		g := ga.currentGainLinear
		for i := range samples {
			samples[i] *= g
		}
		return
	}

	step := (ga.targetGainLinear - ga.currentGainLinear) / float32(n)
	g := ga.currentGainLinear
	for i := range samples {
		g += step
		samples[i] *= g
	}
	ga.currentGainLinear = ga.targetGainLinear
}

func (ga *gainApplier) CurrentGainLinear() float32 {
	return ga.currentGainLinear
}
