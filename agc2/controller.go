package agc2

import "math"

// AdaptiveDigitalGainController tracks speech/noise levels and headroom and computes
// a target gain based on the configured headroom and noise limits. gain changes are
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
	lastSpeechProb    float32
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
		speechEstimator: newSpeechLevelEstimator(config),
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
	c.lastSpeechProb = speechProb
	rms := computeRms(samples)
	rmsDbfs := linearToDb(rms)

	c.speechEstimator.Update(rmsDbfs, speechProb)
	speechLevel := c.speechEstimator.LevelDbfs()

	peakDbfs := computePeakDbfs(samples)
	c.satProtector.Analyze(speechProb, peakDbfs, speechLevel)

	c.noiseEstimator.Update(samples)

	targetGainDb := c.computeTargetGain(speechLevel, c.noiseEstimator.LevelDbfs(), c.satProtector.HeadroomDb())
	limiterEnvelopeDbfs := c.limiterInst.LastAudioLevelDbfs()
	targetGainDb = limitGainByLowConfidence(targetGainDb, c.currentGainDb, limiterEnvelopeDbfs, c.speechEstimator.IsConfident())

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

func (c *AdaptiveDigitalGainController) computeTargetGain(speechLevelDbfs float32, noiseLevelDbfs float32, headroomDb float32) float32 {
	inputLevelDbfs := speechLevelDbfs + headroomDb
	desiredGain := computeGainDb(inputLevelDbfs, c.config)

	if desiredGain < minGainDb {
		desiredGain = minGainDb
	}
	if desiredGain > c.config.MaxGainDb {
		desiredGain = c.config.MaxGainDb
	}

	noiseGainLimit := c.config.MaxOutputNoiseLevelDbfs - noiseLevelDbfs
	if noiseGainLimit < 0 {
		noiseGainLimit = 0
	}
	if desiredGain > noiseGainLimit {
		desiredGain = noiseGainLimit
	}

	return float32(math.Max(float64(desiredGain), float64(minGainDb)))
}

func limitGainByLowConfidence(targetGainDb float32, lastGainDb float32, limiterEnvelopeDbfs float32, estimateIsConfident bool) float32 {
	if estimateIsConfident || limiterEnvelopeDbfs <= limiterThresholdForAgcGainDbfs {
		return targetGainDb
	}

	limiterLevelBeforeGain := limiterEnvelopeDbfs - lastGainDb
	newTargetGainDb := limiterThresholdForAgcGainDbfs - limiterLevelBeforeGain
	if newTargetGainDb < 0 {
		newTargetGainDb = 0
	}
	if newTargetGainDb < targetGainDb {
		return newTargetGainDb
	}
	return targetGainDb
}

func computeGainDb(inputLevelDbfs float32, config AdaptiveDigitalConfig) float32 {
	if inputLevelDbfs < -(config.HeadroomDb+config.MaxGainDb) {
		return config.MaxGainDb
	}
	if inputLevelDbfs < -config.HeadroomDb {
		return -config.HeadroomDb - inputLevelDbfs
	}
	return 0
}

func computePeakDbfs(samples []float32) float32 {
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
	if peak <= 1.0 {
		return minLevelDb
	}
	return linearToDb(peak)
}

// GainDb returns the current adaptive gain in dB as of the last Process call.
func (c *AdaptiveDigitalGainController) GainDb() float32 {
	return c.currentGainDb
}

// SpeechProbability returns the VAD speech probability from the last Process call.
func (c *AdaptiveDigitalGainController) SpeechProbability() float32 {
	return c.lastSpeechProb
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

// SpeechProbability returns the VAD speech probability from the last Process call.
// returns 0 if the adaptive stage is not enabled.
func (gc *GainController2) SpeechProbability() float32 {
	if gc.adaptive != nil {
		return gc.adaptive.SpeechProbability()
	}
	return 0
}

// Reset clears all adaptive state. fixed gain is unaffected.
func (gc *GainController2) Reset() {
	if gc.adaptive != nil {
		gc.adaptive.Reset()
	}
}
