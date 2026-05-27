package agc2

import "math"

const (
	vadConfidenceThreshold                = 0.95
	adjacentSpeechFramesThreshold         = 12
	levelEstimatorTimeToConfidenceMs     = 400
	levelEstimatorLeakFactor             = 0.9975
	saturationProtectorInitialHeadroomDb = 20.0
	maxSpeechLevelDb                      = 30.0

	noiseUpdatePeriodNumFrames = 500
	noiseEstimatorAttack       = 0.5
)

type ratio struct {
	numerator   float32
	denominator float32
}

func (r ratio) value() float32 {
	if r.denominator == 0 {
		return 0
	}
	return r.numerator / r.denominator
}

type levelEstimatorState struct {
	timeToConfidenceMs int
	levelDbfs          ratio
}

type speechLevelEstimator struct {
	initialSpeechLevelDbfs      float32
	adjacentSpeechFramesThreshold int
	preliminaryState            levelEstimatorState
	reliableState               levelEstimatorState
	levelDbfs                   float32
	isConfident                 bool
	numAdjacentSpeechFrames     int
}

func newSpeechLevelEstimator(config AdaptiveDigitalConfig, thresholdOverride ...int) *speechLevelEstimator {
	threshold := adjacentSpeechFramesThreshold
	if len(thresholdOverride) > 0 {
		threshold = thresholdOverride[0]
	}
	if threshold < 1 {
		threshold = 1
	}

	initialLevelDbfs := clampLevelEstimateDbfs(-saturationProtectorInitialHeadroomDb - config.InitialGainDb - config.HeadroomDb)
	est := &speechLevelEstimator{
		initialSpeechLevelDbfs:      initialLevelDbfs,
		adjacentSpeechFramesThreshold: threshold,
	}
	est.Reset()
	return est
}

func (sle *speechLevelEstimator) Update(rmsDbfs float32, speechProbability float32) {
	if speechProbability < vadConfidenceThreshold {
		if sle.adjacentSpeechFramesThreshold > 1 {
			if sle.numAdjacentSpeechFrames >= sle.adjacentSpeechFramesThreshold {
				sle.reliableState = sle.preliminaryState
			} else if sle.numAdjacentSpeechFrames > 0 {
				sle.preliminaryState = sle.reliableState
			}
		}
		sle.numAdjacentSpeechFrames = 0
	} else {
		sle.numAdjacentSpeechFrames++

		bufferFull := sle.preliminaryState.timeToConfidenceMs == 0
		if !bufferFull {
			sle.preliminaryState.timeToConfidenceMs -= frameDurationMs
			if sle.preliminaryState.timeToConfidenceMs < 0 {
				sle.preliminaryState.timeToConfidenceMs = 0
			}
		}

		leakFactor := float32(1.0)
		if bufferFull {
			leakFactor = levelEstimatorLeakFactor
		}
		sle.preliminaryState.levelDbfs.numerator =
			sle.preliminaryState.levelDbfs.numerator*leakFactor + rmsDbfs*speechProbability
		sle.preliminaryState.levelDbfs.denominator =
			sle.preliminaryState.levelDbfs.denominator*leakFactor + speechProbability

		levelDbfs := sle.preliminaryState.levelDbfs.value()
		if sle.numAdjacentSpeechFrames >= sle.adjacentSpeechFramesThreshold {
			sle.levelDbfs = clampLevelEstimateDbfs(levelDbfs)
		}
	}

	sle.updateIsConfident()
}

func (sle *speechLevelEstimator) LevelDbfs() float32 {
	return sle.levelDbfs
}

// Confidence returns 1 if the estimator is confident, 0 otherwise.
func (sle *speechLevelEstimator) Confidence() float32 {
	if sle.isConfident {
		return 1
	}
	return 0
}

func (sle *speechLevelEstimator) IsConfident() bool {
	return sle.isConfident
}

func (sle *speechLevelEstimator) Reset() {
	sle.preliminaryState = sle.makeInitialState()
	sle.reliableState = sle.makeInitialState()
	sle.levelDbfs = sle.initialSpeechLevelDbfs
	sle.isConfident = false
	sle.numAdjacentSpeechFrames = 0
}

func (sle *speechLevelEstimator) updateIsConfident() {
	if sle.adjacentSpeechFramesThreshold == 1 {
		sle.isConfident = sle.preliminaryState.timeToConfidenceMs == 0
		return
	}
	if sle.reliableState.timeToConfidenceMs == 0 {
		sle.isConfident = true
		return
	}
	sle.isConfident = sle.numAdjacentSpeechFrames >= sle.adjacentSpeechFramesThreshold &&
		sle.preliminaryState.timeToConfidenceMs == 0
}

func (sle *speechLevelEstimator) makeInitialState() levelEstimatorState {
	return levelEstimatorState{
		timeToConfidenceMs: levelEstimatorTimeToConfidenceMs,
		levelDbfs: ratio{
			numerator:   sle.initialSpeechLevelDbfs,
			denominator: 1.0,
		},
	}
}

func clampLevelEstimateDbfs(levelDbfs float32) float32 {
	if levelDbfs < minLevelDb {
		return minLevelDb
	}
	if levelDbfs > maxSpeechLevelDb {
		return maxSpeechLevelDb
	}
	return levelDbfs
}

type noiseLevelEstimator struct {
	sampleRateHz                int
	samplesPerChannel           int
	minNoiseEnergy              float32
	firstPeriod                 bool
	preliminaryNoiseEnergySet   bool
	preliminaryNoiseEnergy      float32
	noiseEnergy                 float32
	counter                     int
}

func newNoiseLevelEstimator() *noiseLevelEstimator {
	nle := &noiseLevelEstimator{}
	nle.initialize(48000)
	return nle
}

func (nle *noiseLevelEstimator) Update(samples []float32) {
	if len(samples) == 0 {
		return
	}
	samplesPerChannel := len(samples)
	sampleRateHz := samplesPerChannel * framesPerSecond
	if sampleRateHz != nle.sampleRateHz {
		nle.initialize(sampleRateHz)
	}

	nle.samplesPerChannel = samplesPerChannel

	frameEnergy := frameEnergy(samples)
	if frameEnergy <= nle.minNoiseEnergy {
		return
	}

	if nle.preliminaryNoiseEnergySet {
		if frameEnergy < nle.preliminaryNoiseEnergy {
			nle.preliminaryNoiseEnergy = frameEnergy
		}
	} else {
		nle.preliminaryNoiseEnergy = frameEnergy
		nle.preliminaryNoiseEnergySet = true
	}

	if nle.counter == 0 {
		nle.firstPeriod = false
		nle.noiseEnergy = smoothNoiseFloorEstimate(nle.noiseEnergy, nle.preliminaryNoiseEnergy)
		nle.counter = noiseUpdatePeriodNumFrames
		nle.preliminaryNoiseEnergySet = false
	} else if nle.firstPeriod {
		nle.noiseEnergy = nle.preliminaryNoiseEnergy
		nle.counter--
	} else {
		if nle.preliminaryNoiseEnergy < nle.noiseEnergy {
			nle.noiseEnergy = nle.preliminaryNoiseEnergy
		}
		nle.counter--
	}
}

func (nle *noiseLevelEstimator) LevelDbfs() float32 {
	if nle.samplesPerChannel == 0 {
		return minLevelDb
	}
	return energyToDbfs(nle.noiseEnergy, nle.samplesPerChannel)
}

func (nle *noiseLevelEstimator) Reset() {
	sampleRateHz := nle.sampleRateHz
	if sampleRateHz == 0 {
		sampleRateHz = 48000
	}
	nle.initialize(sampleRateHz)
}

func (nle *noiseLevelEstimator) initialize(sampleRateHz int) {
	nle.sampleRateHz = sampleRateHz
	nle.samplesPerChannel = sampleRateHz / framesPerSecond
	nle.firstPeriod = true
	nle.preliminaryNoiseEnergySet = false
	nle.minNoiseEnergy = float32(sampleRateHz) * 2.0 * 2.0 / float32(framesPerSecond)
	nle.preliminaryNoiseEnergy = nle.minNoiseEnergy
	nle.noiseEnergy = nle.minNoiseEnergy
	nle.counter = noiseUpdatePeriodNumFrames
}

func smoothNoiseFloorEstimate(currentEstimate, newEstimate float32) float32 {
	if currentEstimate < newEstimate {
		return noiseEstimatorAttack*newEstimate + (1.0-noiseEstimatorAttack)*currentEstimate
	}
	return newEstimate
}

func frameEnergy(samples []float32) float32 {
	var energy float32
	for _, s := range samples {
		energy += s * s
	}
	return energy
}

func energyToDbfs(signalEnergy float32, numSamples int) float32 {
	rmsSquare := signalEnergy / float32(numSamples)
	if rmsSquare <= 1.0 {
		return minLevelDb
	}
	return 10.0*float32(math.Log10(float64(rmsSquare))) + minLevelDb
}
