//go:build cgo

package rnnoise

import (
	"math"
	"testing"
)

func TestNew(t *testing.T) {
	d := New()
	if d == nil {
		t.Fatal("New returned nil")
	}
	d.Close()
}

func TestProcessFrameSilence(t *testing.T) {
	d := New()
	defer d.Close()

	frame := make([]float32, FrameSize)
	vad := d.ProcessFrame(frame)

	if vad < 0 || vad > 1 {
		t.Errorf("VAD probability out of range: %f", vad)
	}
	for i, v := range frame {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("frame[%d] = NaN/Inf after processing silence", i)
		}
	}
}

func TestProcessFrameTone(t *testing.T) {
	d := New()
	defer d.Close()

	// warm up with a few frames of tone
	for iter := 0; iter < 20; iter++ {
		frame := make([]float32, FrameSize)
		for i := range frame {
			frame[i] = 1000 * float32(math.Sin(2*math.Pi*440*float64(iter*FrameSize+i)/float64(SampleRate)))
		}
		d.ProcessFrame(frame)
	}

	frame := make([]float32, FrameSize)
	for i := range frame {
		frame[i] = 1000 * float32(math.Sin(2*math.Pi*440*float64(i)/float64(SampleRate)))
	}
	vad := d.ProcessFrame(frame)
	t.Logf("VAD probability for 440Hz tone: %.3f", vad)
}

func TestReset(t *testing.T) {
	d := New()
	defer d.Close()

	frame := make([]float32, FrameSize)
	for i := range frame {
		frame[i] = 500
	}
	d.ProcessFrame(frame)
	d.Reset()

	frame2 := make([]float32, FrameSize)
	d.ProcessFrame(frame2)

	for i, v := range frame2 {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("frame[%d] = NaN/Inf after reset + process", i)
		}
	}
}

func TestVADAdapter(t *testing.T) {
	va := NewVADAdapter()
	defer va.Close()

	// 160-sample 16kHz frame of silence
	frame := make([]float32, 160)
	prob := va.Analyze(frame)

	if prob < 0 || prob > 1 {
		t.Errorf("VAD probability out of range: %f", prob)
	}

	va.Reset()
	prob2 := va.Analyze(frame)
	if prob2 < 0 || prob2 > 1 {
		t.Errorf("VAD probability out of range after reset: %f", prob2)
	}
}

func TestProcessFramePanicsOnShortInput(t *testing.T) {
	d := New()
	defer d.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on short frame")
		}
	}()

	short := make([]float32, 100)
	d.ProcessFrame(short)
}

func BenchmarkProcessFrame(b *testing.B) {
	d := New()
	defer d.Close()

	frame := make([]float32, FrameSize)
	for i := range frame {
		frame[i] = float32(math.Sin(float64(i) * 0.1))
	}
	b.ResetTimer()
	for range b.N {
		d.ProcessFrame(frame)
	}
}

func BenchmarkVADAdapter(b *testing.B) {
	va := NewVADAdapter()
	defer va.Close()

	frame := make([]float32, 160)
	for i := range frame {
		frame[i] = float32(math.Sin(float64(i) * 0.1))
	}
	b.ResetTimer()
	for range b.N {
		va.Analyze(frame)
	}
}
