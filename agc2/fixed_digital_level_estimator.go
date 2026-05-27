package agc2

// fixedDigitalLevelEstimator produces a smoothed peak envelope per sub-frame.
type fixedDigitalLevelEstimator struct {
	filterStateLevel float32
	samplesInFrame   int
	samplesInSubFrame int
}

func newFixedDigitalLevelEstimator(samplesPerChannel int) *fixedDigitalLevelEstimator {
	est := &fixedDigitalLevelEstimator{}
	est.SetSamplesPerChannel(samplesPerChannel)
	return est
}

func (e *fixedDigitalLevelEstimator) SetSamplesPerChannel(samplesPerChannel int) {
	e.samplesInFrame = samplesPerChannel
	if samplesPerChannel <= 0 {
		e.samplesInSubFrame = 0
		return
	}
	e.samplesInSubFrame = samplesPerChannel / limiterSubFrames
	if e.samplesInSubFrame <= 0 {
		e.samplesInSubFrame = 1
	}
}

// ComputeLevel fills envelope with sub-frame peak estimates (length limiterSubFrames).
func (e *fixedDigitalLevelEstimator) ComputeLevel(samples []float32, envelope []float32) {
	if len(samples) == 0 || len(envelope) < limiterSubFrames {
		return
	}
	if e.samplesInFrame != len(samples) {
		e.SetSamplesPerChannel(len(samples))
	}
	subFrameLen := e.samplesInSubFrame
	if subFrameLen <= 0 {
		return
	}

	for i := 0; i < limiterSubFrames; i++ {
		start := i * subFrameLen
		end := start + subFrameLen
		if start >= len(samples) {
			envelope[i] = 0
			continue
		}
		if i == limiterSubFrames-1 || end > len(samples) {
			end = len(samples)
		}
		var max float32
		for _, s := range samples[start:end] {
			abs := s
			if abs < 0 {
				abs = -abs
			}
			if abs > max {
				max = abs
			}
		}
		envelope[i] = max
	}

	for i := 0; i < limiterSubFrames-1; i++ {
		if envelope[i] < envelope[i+1] {
			envelope[i] = envelope[i+1]
		}
	}

	for i := 0; i < limiterSubFrames; i++ {
		env := envelope[i]
		if env > e.filterStateLevel {
			env = env*(1.0-fixedDigitalLevelEstimatorAttack) + e.filterStateLevel*fixedDigitalLevelEstimatorAttack
		} else {
			env = env*(1.0-fixedDigitalLevelEstimatorDecay) + e.filterStateLevel*fixedDigitalLevelEstimatorDecay
		}
		envelope[i] = env
		e.filterStateLevel = env
	}
}

func (e *fixedDigitalLevelEstimator) Reset() {
	e.filterStateLevel = 0
}

func (e *fixedDigitalLevelEstimator) LastAudioLevel() float32 {
	return e.filterStateLevel
}
