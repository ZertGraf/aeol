package agc2

import "math"

type fixedDigitalLevelEstimator struct {
	smoothedLevel float32
	decayRate     float32
}

func newFixedDigitalLevelEstimator() *fixedDigitalLevelEstimator {
	return &fixedDigitalLevelEstimator{
		smoothedLevel: 0,
		decayRate:     0.9993,
	}
}

func (e *fixedDigitalLevelEstimator) ComputeLevel(samples []float32) float32 {
	var peak float32
	for _, s := range samples {
		abs := s
		if abs < 0 {
			abs = -abs
		}
		if abs > peak {
			peak = abs
		}
	}

	if peak > e.smoothedLevel {
		e.smoothedLevel = peak
	} else {
		e.smoothedLevel = e.decayRate*e.smoothedLevel + (1-e.decayRate)*peak
	}

	return e.smoothedLevel
}

func (e *fixedDigitalLevelEstimator) Reset() {
	e.smoothedLevel = 0
}

type speechLevelEstimator struct {
	levelDbfs       float32
	confidence      float32
	initialLevel    float32
	updateRate      float32
	minConfidence   float32
}

func newSpeechLevelEstimator(initialLevelDbfs float32) *speechLevelEstimator {
	return &speechLevelEstimator{
		levelDbfs:     initialLevelDbfs,
		confidence:    0.0,
		initialLevel:  initialLevelDbfs,
		updateRate:    0.01,
		minConfidence: 0.2,
	}
}

func (sle *speechLevelEstimator) Update(rmsDbfs float32, speechProbability float32) {
	if speechProbability < 0.5 {
		return
	}

	alpha := sle.updateRate * speechProbability
	sle.levelDbfs = (1-alpha)*sle.levelDbfs + alpha*rmsDbfs

	confidenceIncrease := float32(0.01) * speechProbability
	sle.confidence += confidenceIncrease
	if sle.confidence > 1 {
		sle.confidence = 1
	}
}

func (sle *speechLevelEstimator) LevelDbfs() float32 {
	return sle.levelDbfs
}

func (sle *speechLevelEstimator) Confidence() float32 {
	return sle.confidence
}

func (sle *speechLevelEstimator) Reset() {
	sle.levelDbfs = sle.initialLevel
	sle.confidence = 0
}

type noiseLevelEstimator struct {
	levelDbfs    float32
	initialized  bool
	updateRate   float32
}

func newNoiseLevelEstimator() *noiseLevelEstimator {
	return &noiseLevelEstimator{
		levelDbfs:  minLevelDb,
		updateRate: 0.001,
	}
}

func (nle *noiseLevelEstimator) Update(rmsDbfs float32, speechProbability float32) {
	if speechProbability > 0.5 {
		return
	}

	if !nle.initialized {
		nle.levelDbfs = rmsDbfs
		nle.initialized = true
		return
	}

	noiseProbability := 1.0 - speechProbability
	alpha := nle.updateRate * float32(math.Min(float64(noiseProbability), 0.5))
	nle.levelDbfs = (1-alpha)*nle.levelDbfs + alpha*rmsDbfs
}

func (nle *noiseLevelEstimator) LevelDbfs() float32 {
	return nle.levelDbfs
}

func (nle *noiseLevelEstimator) Reset() {
	nle.levelDbfs = minLevelDb
	nle.initialized = false
}
