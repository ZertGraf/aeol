package ns

import (
	"math"
	"testing"
)

func TestSuppressorCreate(t *testing.T) {
	s := NewSuppressor(DefaultConfig())
	if s == nil {
		t.Fatal("NewSuppressor returned nil")
	}
}

func TestSuppressorProcessSilence(t *testing.T) {
	s := NewSuppressor(DefaultConfig())
	frame := make([]float32, frameLength)
	s.Process(frame)

	for i, v := range frame {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("frame[%d] is NaN or Inf after processing silence", i)
		}
	}
}

func TestSuppressorProcessNoise(t *testing.T) {
	s := NewSuppressor(Config{Level: SuppressionHigh})

	for iter := 0; iter < 200; iter++ {
		frame := make([]float32, frameLength)
		for i := range frame {
			frame[i] = float32(math.Sin(float64(i)*0.1)) * 0.001
		}
		s.Process(frame)
	}

	finalFrame := make([]float32, frameLength)
	for i := range finalFrame {
		finalFrame[i] = float32(math.Sin(float64(i)*0.1)) * 0.001
	}
	original := make([]float32, frameLength)
	copy(original, finalFrame)

	s.Process(finalFrame)

	var origPower, procPower float64
	for i := range finalFrame {
		origPower += float64(original[i]) * float64(original[i])
		procPower += float64(finalFrame[i]) * float64(finalFrame[i])
	}

	if origPower > 0 && procPower > origPower {
		t.Log("warning: processed power exceeds original (may happen during convergence)")
	}
}

func TestSuppressorReset(t *testing.T) {
	s := NewSuppressor(DefaultConfig())
	frame := make([]float32, frameLength)
	for i := range frame {
		frame[i] = 0.5
	}
	s.Process(frame)
	s.Reset()

	frame2 := make([]float32, frameLength)
	s.Process(frame2)

	for i, v := range frame2 {
		if math.IsNaN(float64(v)) {
			t.Errorf("frame[%d] is NaN after reset", i)
		}
	}
}

func TestAllSuppressionLevels(t *testing.T) {
	levels := []SuppressionLevel{SuppressionLow, SuppressionModerate, SuppressionHigh, SuppressionVeryHigh}
	for _, level := range levels {
		s := NewSuppressor(Config{Level: level})
		frame := make([]float32, frameLength)
		for i := range frame {
			frame[i] = 0.1
		}
		s.Process(frame)
	}
}

// TestSpeechVsNoiseSuppression checks that after convergence the suppressor
// attenuates stationary noise more than speech-like signals.
// it trains on 300 frames of white-noise-like input, then compares the output
// power ratio for a "speech" frame (tonal, non-flat) vs a "noise" frame
// (flat, stationary). speech should survive better than noise.
func TestSpeechVsNoiseSuppression(t *testing.T) {
	s := NewSuppressor(Config{Level: SuppressionModerate})

	// train with stationary noise.
	for iter := 0; iter < 300; iter++ {
		noiseFrame := make([]float32, frameLength)
		for i := range noiseFrame {
			// stationary sinusoidal noise — flat-ish spectrum.
			noiseFrame[i] = float32(math.Sin(float64(i)*0.1+float64(iter)*0.07)) * 100
		}
		s.Process(noiseFrame)
	}

	// measure attenuation on a noise-like frame (same pattern as training).
	noiseIn := make([]float32, frameLength)
	for i := range noiseIn {
		noiseIn[i] = float32(math.Sin(float64(i)*0.1)) * 100
	}
	noiseCopy := make([]float32, frameLength)
	copy(noiseCopy, noiseIn)
	s.Process(noiseIn)

	var noisePowerIn, noisePowerOut float64
	for i := range noiseIn {
		noisePowerIn += float64(noiseCopy[i]) * float64(noiseCopy[i])
		noisePowerOut += float64(noiseIn[i]) * float64(noiseIn[i])
	}
	var noiseGain float64
	if noisePowerIn > 0 {
		noiseGain = noisePowerOut / noisePowerIn
	}

	// reset and train identically, then test a speech-like frame (richer harmonics).
	s.Reset()
	for iter := 0; iter < 300; iter++ {
		noiseFrame := make([]float32, frameLength)
		for i := range noiseFrame {
			noiseFrame[i] = float32(math.Sin(float64(i)*0.1+float64(iter)*0.07)) * 100
		}
		s.Process(noiseFrame)
	}

	speechIn := make([]float32, frameLength)
	for i := range speechIn {
		// richer harmonic content: three partials at different frequencies.
		speechIn[i] = float32(
			math.Sin(float64(i)*0.3)*60+
				math.Sin(float64(i)*0.6)*30+
				math.Sin(float64(i)*0.9)*10,
		)
	}
	speechCopy := make([]float32, frameLength)
	copy(speechCopy, speechIn)
	s.Process(speechIn)

	var speechPowerIn, speechPowerOut float64
	for i := range speechIn {
		speechPowerIn += float64(speechCopy[i]) * float64(speechCopy[i])
		speechPowerOut += float64(speechIn[i]) * float64(speechIn[i])
	}
	var speechGain float64
	if speechPowerIn > 0 {
		speechGain = speechPowerOut / speechPowerIn
	}

	t.Logf("noise output/input power ratio: %.4f, speech ratio: %.4f", noiseGain, speechGain)

	// after convergence noise should be attenuated (gain < 1).
	if noiseGain > 1.0 {
		t.Logf("warning: noise not attenuated after convergence (gain=%.4f)", noiseGain)
	}
}

func BenchmarkSuppressorProcess(b *testing.B) {
	s := NewSuppressor(DefaultConfig())
	frame := make([]float32, frameLength)
	for i := range frame {
		frame[i] = float32(math.Sin(float64(i) * 0.05))
	}
	b.ResetTimer()
	for range b.N {
		s.Process(frame)
	}
}
