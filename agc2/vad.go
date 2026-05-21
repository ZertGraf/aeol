package agc2

import "math"

type VoiceActivityDetector struct {
	speechProbability float32
	rmsThresholdDbfs  float32
	hangoverFrames    int
	hangoverCounter   int
}

func NewVoiceActivityDetector() *VoiceActivityDetector {
	return &VoiceActivityDetector{
		rmsThresholdDbfs: -50.0,
		hangoverFrames:   5,
	}
}

func (vad *VoiceActivityDetector) Analyze(samples []float32) float32 {
	rms := computeRms(samples)
	rmsDbfs := linearToDb(rms)

	if rmsDbfs > vad.rmsThresholdDbfs {
		vad.speechProbability = 0.7 + 0.3*sigmoid(rmsDbfs-vad.rmsThresholdDbfs, 5)
		vad.hangoverCounter = vad.hangoverFrames
	} else if vad.hangoverCounter > 0 {
		vad.hangoverCounter--
		vad.speechProbability *= 0.95
	} else {
		vad.speechProbability *= 0.8
		if vad.speechProbability < 0.01 {
			vad.speechProbability = 0
		}
	}

	return vad.speechProbability
}

func (vad *VoiceActivityDetector) SpeechProbability() float32 {
	return vad.speechProbability
}

func (vad *VoiceActivityDetector) Reset() {
	vad.speechProbability = 0
	vad.hangoverCounter = 0
}

func computeRms(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return float32(math.Sqrt(sum / float64(len(samples))))
}

func sigmoid(x float32, steepness float32) float32 {
	return float32(1.0 / (1.0 + math.Exp(-float64(steepness*x))))
}
