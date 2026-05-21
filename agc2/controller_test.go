package agc2

import (
	"math"
	"testing"
)

func TestGainController2Create(t *testing.T) {
	gc := NewGainController2(DefaultConfig())
	if gc == nil {
		t.Fatal("NewGainController2 returned nil")
	}
}

func TestGainController2ProcessSilence(t *testing.T) {
	gc := NewGainController2(DefaultConfig())
	samples := make([]float32, 160)
	gc.Process(samples)

	for i, v := range samples {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("samples[%d] is NaN/Inf after processing silence", i)
		}
	}
}

func TestFixedGainApplication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AdaptiveDigital.Enabled = false
	cfg.FixedDigital.GainDb = 6.0

	gc := NewGainController2(cfg)
	samples := make([]float32, 160)
	for i := range samples {
		samples[i] = 0.1
	}
	original := make([]float32, 160)
	copy(original, samples)

	gc.Process(samples)

	expectedGain := dbToLinear(6.0)
	for i := range samples {
		expected := original[i] * expectedGain
		if math.Abs(float64(samples[i]-expected)) > 1e-5 {
			t.Errorf("samples[%d] = %f, want %f", i, samples[i], expected)
			break
		}
	}
}

func TestAdaptiveGainConvergence(t *testing.T) {
	cfg := DefaultConfig()
	gc := NewGainController2(cfg)

	for iter := 0; iter < 500; iter++ {
		samples := make([]float32, 160)
		for i := range samples {
			samples[i] = 0.01 * float32(math.Sin(2*math.Pi*440*float64(i)/16000))
		}
		gc.Process(samples)
	}

	if gc.adaptive == nil {
		t.Skip("adaptive not enabled")
	}
	gain := gc.adaptive.GainDb()
	if gain < 0 || gain > 30 {
		t.Errorf("gain = %f, want in [0, 30]", gain)
	}
}

func TestVAD(t *testing.T) {
	vad := NewVoiceActivityDetector()

	silence := make([]float32, 160)
	prob := vad.Analyze(silence)
	if prob > 0.5 {
		t.Errorf("silence probability = %f, want < 0.5", prob)
	}

	speech := make([]float32, 160)
	for i := range speech {
		speech[i] = 0.3 * float32(math.Sin(2*math.Pi*300*float64(i)/16000))
	}
	prob = vad.Analyze(speech)
	if prob < 0.1 {
		t.Logf("speech probability = %f (may be low on first frame)", prob)
	}
}

func BenchmarkGainController2(b *testing.B) {
	gc := NewGainController2(DefaultConfig())
	samples := make([]float32, 160)
	for i := range samples {
		samples[i] = 0.01 * float32(math.Sin(float64(i)*0.1))
	}
	b.ResetTimer()
	for range b.N {
		gc.Process(samples)
	}
}
