package agc2

import "math"

// AdaptiveDigitalGainController tracks speech and noise levels via exponential estimators
// and computes a target gain to bring speech toward -18 dBFS. gain changes are
// rate-limited per MaxGainChangeDbPerSecond and a hard limiter clips at FloatS16 bounds.
type AdaptiveDigitalGainController struct {
	config            AdaptiveDigitalConfig
	gainApplier       *gainApplier
	speechEstimator   *speechLevelEstimator
	noiseEstimator    *noiseLevelEstimator
	satProtector      *saturationProtector
	vad               VADAnalyzer
	limiterInst       *limiter
	currentGainDb     float32
}

// NewAdaptiveDigitalGainController creates the adaptive gain stage with the given config.
// InitialGainDb sets the starting gain before any speech has been observed.
// an optional VADAnalyzer overrides the default energy-based VAD.
func NewAdaptiveDigitalGainController(config AdaptiveDigitalConfig, vad ...VADAnalyzer) *AdaptiveDigitalGainController {
	var v VADAnalyzer = NewVoiceActivityDetector()
	if len(vad) > 0 && vad[0] != nil {
		v = vad[0]
	}
	return &AdaptiveDigitalGainController{
		config:          config,
		gainApplier:     newGainApplier(config.InitialGainDb),
		speechEstimator: newSpeechLevelEstimator(-30),
		noiseEstimator:  newNoiseLevelEstimator(),
		satProtector:    newSaturationProtector(),
		vad:             v,
		limiterInst:     newLimiter(),
		currentGainDb:   config.InitialGainDb,
	}
}

// Process applies adaptive gain to samples in-place. samples must be in FloatS16 format.
// runs VAD, updates speech/noise estimators, clamps gain to configured limits, then applies
// a ramped gain and hard limiter. no-op when DryRun is set.
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

// GainDb returns the current adaptive gain in dB as of the last Process call.
func (c *AdaptiveDigitalGainController) GainDb() float32 {
	return c.currentGainDb
}

// Reset returns the controller to its initial state, discarding all learned speech and noise levels.
func (c *AdaptiveDigitalGainController) Reset() {
	c.currentGainDb = c.config.InitialGainDb
	c.gainApplier = newGainApplier(c.config.InitialGainDb)
	c.speechEstimator.Reset()
	c.noiseEstimator.Reset()
	c.satProtector.Reset()
	c.vad.Reset()
	c.limiterInst.Reset()
}

// GainController2 is the top-level AGC2 processor. it applies a fixed linear gain
// first, then an optional adaptive gain stage. operates in-place on FloatS16 frames of any size.
type GainController2 struct {
	config     Config
	adaptive   *AdaptiveDigitalGainController
	fixedGain  float32
}

// NewGainController2 creates a GainController2 from the given config.
// FixedDigital.GainDb of 0 means unity (no fixed gain). set AdaptiveDigital.Enabled to
// activate the adaptive stage; otherwise only the fixed gain is applied.
// an optional VADAnalyzer overrides the default energy-based VAD used by the adaptive stage.
func NewGainController2(config Config, vad ...VADAnalyzer) *GainController2 {
	gc := &GainController2{
		config:    config,
		fixedGain: dbToLinear(config.FixedDigital.GainDb),
	}
	if config.AdaptiveDigital.Enabled {
		gc.adaptive = NewAdaptiveDigitalGainController(config.AdaptiveDigital, vad...)
	}
	return gc
}

// Process applies gain to samples in-place. samples must be in FloatS16 format (float32 in [-32768, 32767]).
// fixed gain is applied first as a scalar multiply, then the adaptive stage runs if enabled.
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

// Reset clears all adaptive state. fixed gain is unaffected.
func (gc *GainController2) Reset() {
	if gc.adaptive != nil {
		gc.adaptive.Reset()
	}
}
