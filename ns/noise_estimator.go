package ns

type quantileState struct {
	density  [numFreqBins]float32
	logQuant [numFreqBins]float32
	counter  int
	order    float32
}

func newQuantileState(order float32) *quantileState {
	qs := &quantileState{order: order}
	for i := range qs.density {
		qs.density[i] = 0.3
	}
	for i := range qs.logQuant {
		qs.logQuant[i] = 8.0
	}
	return qs
}

type quantileNoiseEstimator struct {
	states [3]*quantileState
}

func newQuantileNoiseEstimator() *quantileNoiseEstimator {
	return &quantileNoiseEstimator{
		states: [3]*quantileState{
			newQuantileState(0.25),
			newQuantileState(0.25),
			newQuantileState(0.25),
		},
	}
}

func (q *quantileNoiseEstimator) Estimate(logSpectrum [numFreqBins]float32, noiseSpectrum []float32) {
	for stateIdx, state := range q.states {
		updateInterval := 1
		switch stateIdx {
		case 1:
			updateInterval = 2
		case 2:
			updateInterval = 4
		}

		state.counter++
		if state.counter < updateInterval {
			continue
		}
		state.counter = 0

		for i := 0; i < numFreqBins; i++ {
			lrt := logSpectrum[i]
			delta := (lrt - state.logQuant[i]) * state.density[i]
			state.logQuant[i] += delta

			if delta > 0 {
				state.density[i] = (1-state.order)*state.density[i] + state.order*0.4
			} else {
				state.density[i] = (1-state.order)*state.density[i] + state.order*0.3
			}

			if state.density[i] < 0.01 {
				state.density[i] = 0.01
			}
			if state.density[i] > 1.0 {
				state.density[i] = 1.0
			}
		}
	}

	for i := 0; i < numFreqBins; i++ {
		noiseSpectrum[i] = q.states[0].logQuant[i]
		for _, state := range q.states[1:] {
			if state.logQuant[i] < noiseSpectrum[i] {
				noiseSpectrum[i] = state.logQuant[i]
			}
		}
	}
}

type noiseEstimator struct {
	quantile    *quantileNoiseEstimator
	noiseSpec   [numFreqBins]float32
	prevNoise   [numFreqBins]float32
	whiteNoise  float32
	pinkNoise   float32
	initialized bool
}

func newNoiseEstimator() *noiseEstimator {
	ne := &noiseEstimator{
		quantile:  newQuantileNoiseEstimator(),
		whiteNoise: 0,
		pinkNoise:  0,
	}
	for i := range ne.noiseSpec {
		ne.noiseSpec[i] = 1.0
	}
	return ne
}

func (ne *noiseEstimator) Update(logSpectrum [numFreqBins]float32) {
	ne.quantile.Estimate(logSpectrum, ne.noiseSpec[:])

	if !ne.initialized {
		copy(ne.prevNoise[:], ne.noiseSpec[:])
		ne.initialized = true
	}

	var sumNoise float64
	for i := 0; i < numFreqBins; i++ {
		ne.noiseSpec[i] = 0.9*ne.prevNoise[i] + 0.1*ne.noiseSpec[i]
		sumNoise += float64(ne.noiseSpec[i])
	}
	ne.whiteNoise = float32(sumNoise / numFreqBins)

	copy(ne.prevNoise[:], ne.noiseSpec[:])
}

func (ne *noiseEstimator) Spectrum() [numFreqBins]float32 {
	return ne.noiseSpec
}
