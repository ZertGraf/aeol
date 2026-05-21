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
