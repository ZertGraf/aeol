package agc2

import "math"

type AdaptiveDigitalGainController struct {
	config            AdaptiveDigitalConfig
	gainApplier       *gainApplier
	speechEstimator   *speechLevelEstimator
	noiseEstimator    *noiseLevelEstimator
	satProtector      *saturationProtector
	vad               *VoiceActivityDetector
	limiterInst       *limiter
	currentGainDb     float32
}

func NewAdaptiveDigitalGainController(config AdaptiveDigitalConfig) *AdaptiveDigitalGainController {
	return &AdaptiveDigitalGainController{
		config:          config,
		gainApplier:     newGainApplier(config.InitialGainDb),
		speechEstimator: newSpeechLevelEstimator(-30),
		noiseEstimator:  newNoiseLevelEstimator(),
		satProtector:    newSaturationProtector(),
		vad:             NewVoiceActivityDetector(),
		limiterInst:     newLimiter(),
		currentGainDb:   config.InitialGainDb,
	}
}

func (c *AdaptiveDigitalGainController) Process(samples []float32) {
	speechProb := c.vad.Analyze(samples)
	rms := computeRms(samples)
	rmsDbfs := linearToDb(rms)

	c.speechEstimator.Update(rmsDbfs, speechProb)
	c.noiseEstimator.Update(rmsDbfs, speechProb)

	targetGainDb := c.computeTargetGain()

	maxGainChange := c.config.MaxGainChangeDbPerSecond / float32(framesPerSecond)
	if targetGainDb > c.currentGainDb+maxGainChange {
		targetGainDb = c.currentGainDb + maxGainChange
	} else if targetGainDb < c.currentGainDb-maxGainChange {
		targetGainDb = c.currentGainDb - maxGainChange
	}

	c.currentGainDb = targetGainDb

	if !c.config.DryRun {
		c.gainApplier.SetGainDb(c.currentGainDb)
		c.gainApplier.Apply(samples)
		c.limiterInst.Process(samples)
	}
}

func (c *AdaptiveDigitalGainController) computeTargetGain() float32 {
	speechLevel := c.speechEstimator.LevelDbfs()
	noiseLevel := c.noiseEstimator.LevelDbfs()

	targetOutputDbfs := float32(-18.0)
	desiredGain := targetOutputDbfs - speechLevel

	if desiredGain < minGainDb {
		desiredGain = minGainDb
	}
	if desiredGain > c.config.MaxGainDb {
		desiredGain = c.config.MaxGainDb
	}

	noiseGainLimit := c.config.MaxOutputNoiseLevelDbfs - noiseLevel
	if noiseGainLimit < 0 {
		noiseGainLimit = 0
	}
	if desiredGain > noiseGainLimit {
		desiredGain = noiseGainLimit
	}

	desiredGain -= c.config.HeadroomDb

	return float32(math.Max(float64(desiredGain), float64(minGainDb)))
}

func (c *AdaptiveDigitalGainController) GainDb() float32 {
	return c.currentGainDb
}

func (c *AdaptiveDigitalGainController) Reset() {
	c.currentGainDb = c.config.InitialGainDb
	c.gainApplier = newGainApplier(c.config.InitialGainDb)
	c.speechEstimator.Reset()
	c.noiseEstimator.Reset()
	c.satProtector.Reset()
	c.vad.Reset()
	c.limiterInst.Reset()
}

type GainController2 struct {
	config     Config
	adaptive   *AdaptiveDigitalGainController
	fixedGain  float32
}

func NewGainController2(config Config) *GainController2 {
	gc := &GainController2{
		config:    config,
		fixedGain: dbToLinear(config.FixedDigital.GainDb),
	}
	if config.AdaptiveDigital.Enabled {
		gc.adaptive = NewAdaptiveDigitalGainController(config.AdaptiveDigital)
	}
	return gc
}

func (gc *GainController2) Process(samples []float32) {
	if gc.fixedGain != 1.0 {
		for i := range samples {
			samples[i] *= gc.fixedGain
		}
	}

	if gc.adaptive != nil {
		gc.adaptive.Process(samples)
	}
}

func (gc *GainController2) Reset() {
	if gc.adaptive != nil {
		gc.adaptive.Reset()
	}
}
