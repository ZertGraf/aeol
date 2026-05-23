package agc2

import "math"

type limiterDbGainCurve struct {
	kneeStartDbfs       float32
	limiterStartDbfs    float32
	compressionRatio    float32
	maxInputLevelDbfs   float32
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

	return -(inputDbfs - c.limiterStartDbfs) + (c.maxInputLevelDbfs-c.limiterStartDbfs)/c.compressionRatio
}

type limiter struct {
	curve         *limiterDbGainCurve
	levelEstimator *fixedDigitalLevelEstimator
}

func newLimiter() *limiter {
	return &limiter{
		curve:         newLimiterDbGainCurve(),
		levelEstimator: newFixedDigitalLevelEstimator(),
	}
}

func (l *limiter) Process(samples []float32) {
	level := l.levelEstimator.ComputeLevel(samples)
	if level < 1e-10 {
		return
	}

	levelDbfs := linearToDb(level)
	gainDb := l.curve.GainDb(levelDbfs)

	if gainDb >= 0 {
		return
	}

	gainLinear := dbToLinear(gainDb)
	for i := range samples {
		samples[i] *= gainLinear
		if samples[i] > 32767.0 {
			samples[i] = 32767.0
		} else if samples[i] < -32768.0 {
			samples[i] = -32768.0
		}
	}
}

func (l *limiter) Reset() {
	l.levelEstimator.Reset()
}

type saturationProtector struct {
	marginDb       float32
	peakEnvelop    float32
	decayRate      float32
}

func newSaturationProtector() *saturationProtector {
	return &saturationProtector{
		marginDb:    2.0,
		peakEnvelop: 0,
		decayRate:   0.9993,
	}
}

func (sp *saturationProtector) HeadroomDb(samples []float32, preGainDb float32) float32 {
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

	if peak > sp.peakEnvelop {
		sp.peakEnvelop = peak
	} else {
		sp.peakEnvelop *= sp.decayRate
	}

	peakDbfs := linearToDb(sp.peakEnvelop)
	headroom := -(peakDbfs + preGainDb)
	return float32(math.Max(float64(headroom), 0))
}

func (sp *saturationProtector) Reset() {
	sp.peakEnvelop = 0
}
