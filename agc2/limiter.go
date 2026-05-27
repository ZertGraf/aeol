package agc2

import "math"

type limiterDbGainCurve struct {
	kneeStartDbfs     float32
	limiterStartDbfs  float32
	compressionRatio  float32
	maxInputLevelDbfs float32
}

func newLimiterDbGainCurve() *limiterDbGainCurve {
	return &limiterDbGainCurve{
		kneeStartDbfs:    -2.0,
		limiterStartDbfs: -1.0,
		compressionRatio: 5.0,
		maxInputLevelDbfs: 2.0,
	}
}

func (c *limiterDbGainCurve) GainDb(inputDbfs float32) float32 {
	if inputDbfs <= c.kneeStartDbfs {
		return 0
	}

	if inputDbfs <= c.limiterStartDbfs {
		t := (inputDbfs - c.kneeStartDbfs) / (c.limiterStartDbfs - c.kneeStartDbfs)
		return -0.5 * t * t * (c.limiterStartDbfs - c.kneeStartDbfs) / c.compressionRatio
	}

	if inputDbfs <= c.maxInputLevelDbfs {
		excess := inputDbfs - c.limiterStartDbfs
		return -(excess * (1.0 - 1.0/c.compressionRatio))
	}

	return -(inputDbfs-c.limiterStartDbfs) + (c.maxInputLevelDbfs-c.limiterStartDbfs)/c.compressionRatio
}

type limiter struct {
	gainCurve          *interpolatedGainCurve
	levelEstimator     *fixedDigitalLevelEstimator
	levelEnvelope      [limiterSubFrames]float32
	scalingFactors     [limiterSubFrames + 1]float32
	perSampleScaling   []float32
	lastScalingFactor  float32
	samplesPerChannel  int
}

func newLimiter() *limiter {
	return &limiter{
		gainCurve:         newInterpolatedGainCurve(),
		lastScalingFactor: 1.0,
	}
}

func (l *limiter) Process(samples []float32) {
	if len(samples) == 0 {
		return
	}
	if len(samples)%limiterSubFrames != 0 {
		clampSamples(samples)
		return
	}
	l.ensureBuffers(len(samples))

	l.levelEstimator.ComputeLevel(samples, l.levelEnvelope[:])

	l.scalingFactors[0] = l.lastScalingFactor
	for i := 0; i < limiterSubFrames; i++ {
		l.scalingFactors[i+1] = l.gainCurve.LookUpGainToApply(l.levelEnvelope[i])
	}

	subFrameLen := len(samples) / limiterSubFrames
	computePerSampleSubframeFactors(l.scalingFactors[:], l.perSampleScaling, subFrameLen)
	scaleSamples(l.perSampleScaling, samples)

	l.lastScalingFactor = l.scalingFactors[limiterSubFrames]
}

func (l *limiter) ensureBuffers(samplesPerChannel int) {
	if l.levelEstimator == nil {
		l.levelEstimator = newFixedDigitalLevelEstimator(samplesPerChannel)
		l.samplesPerChannel = samplesPerChannel
		l.perSampleScaling = make([]float32, samplesPerChannel)
		return
	}
	if l.samplesPerChannel != samplesPerChannel {
		l.samplesPerChannel = samplesPerChannel
		l.levelEstimator.SetSamplesPerChannel(samplesPerChannel)
		l.perSampleScaling = make([]float32, samplesPerChannel)
	}
}

func (l *limiter) Reset() {
	if l.levelEstimator != nil {
		l.levelEstimator.Reset()
	}
}

func (l *limiter) LastAudioLevelDbfs() float32 {
	if l.levelEstimator == nil {
		return minLevelDb
	}
	level := l.levelEstimator.LastAudioLevel()
	if level <= 0 {
		return minLevelDb
	}
	dbfs := linearToDb(level)
	if dbfs < minLevelDb {
		return minLevelDb
	}
	return dbfs
}

func interpolateFirstSubframe(lastFactor, currentFactor float32, subframe []float32) {
	n := float32(len(subframe))
	if n == 0 {
		return
	}
	for i := range subframe {
		t := float32(i) / n
		weight := math.Pow(float64(1.0-t), attackFirstSubframeInterpolationPower)
		subframe[i] = float32(weight)*(lastFactor-currentFactor) + currentFactor
	}
}

func computePerSampleSubframeFactors(scalingFactors []float32, perSample []float32, subFrameLen int) {
	if subFrameLen <= 0 || len(perSample) == 0 {
		return
	}
	numSubframes := len(scalingFactors) - 1
	isAttack := scalingFactors[0] > scalingFactors[1]
	offset := 0
	start := 0
	if isAttack {
		interpolateFirstSubframe(scalingFactors[0], scalingFactors[1], perSample[:subFrameLen])
		offset = subFrameLen
		start = 1
	}
	for i := start; i < numSubframes; i++ {
		scalingStart := scalingFactors[i]
		scalingEnd := scalingFactors[i+1]
		scalingDiff := (scalingEnd - scalingStart) / float32(subFrameLen)
		for j := 0; j < subFrameLen; j++ {
			perSample[offset+j] = scalingStart + scalingDiff*float32(j)
		}
		offset += subFrameLen
	}
}

func scaleSamples(perSample []float32, samples []float32) {
	if len(perSample) < len(samples) {
		return
	}
	for i, factor := range perSample[:len(samples)] {
		v := samples[i] * factor
		if v > maxFloatS16 {
			v = maxFloatS16
		} else if v < minFloatS16 {
			v = minFloatS16
		}
		samples[i] = v
	}
}

func clampSamples(samples []float32) {
	for i := range samples {
		v := samples[i]
		if v > maxFloatS16 {
			v = maxFloatS16
		} else if v < minFloatS16 {
			v = minFloatS16
		}
		samples[i] = v
	}
}
