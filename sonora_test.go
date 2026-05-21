package sonora

import (
	"math"
	"testing"
)

func TestBuilderDefault(t *testing.T) {
	ap, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer ap.Close()
}

func TestBuilderWithAllModules(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(48000).
		Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(DefaultNsConfig()).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer ap.Close()
}

func TestProcessCaptureFloat(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	frame := make([]float32, 160)
	for i := range frame {
		frame[i] = 0.5 * float32(math.Sin(2*math.Pi*440*float64(i)/16000))
	}
	data := [][]float32{frame}

	err = ap.ProcessCaptureFloat(data)
	if err != nil {
		t.Fatalf("ProcessCaptureFloat failed: %v", err)
	}
}

func TestProcessRenderFloat(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	frame := make([]float32, 160)
	for i := range frame {
		frame[i] = 0.3 * float32(math.Sin(2*math.Pi*300*float64(i)/16000))
	}
	data := [][]float32{frame}

	err = ap.ProcessRenderFloat(data)
	if err != nil {
		t.Fatalf("ProcessRenderFloat failed: %v", err)
	}
}

func TestProcessCaptureInt16(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	frame := make([]int16, 160)
	for i := range frame {
		frame[i] = int16(16000 * math.Sin(2*math.Pi*440*float64(i)/16000))
	}

	err = ap.ProcessCaptureInt16(frame)
	if err != nil {
		t.Fatalf("ProcessCaptureInt16 failed: %v", err)
	}
}

func TestProcessWithNoiseSuppression(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	for iter := 0; iter < 100; iter++ {
		frame := make([]float32, 160)
		for i := range frame {
			frame[i] = 0.01 * float32(math.Sin(float64(i)*0.1))
		}
		data := [][]float32{frame}
		if err := ap.ProcessCaptureFloat(data); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProcessWithAGC(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	for iter := 0; iter < 100; iter++ {
		frame := make([]float32, 160)
		for i := range frame {
			frame[i] = 0.01 * float32(math.Sin(2*math.Pi*440*float64(i)/16000))
		}
		data := [][]float32{frame}
		if err := ap.ProcessCaptureFloat(data); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStatistics(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	frame := make([]float32, 160)
	for i := range frame {
		frame[i] = 0.5
	}
	ap.ProcessCaptureFloat([][]float32{frame})

	stats := ap.Statistics()
	if stats.OutputRmsDbfs == nil {
		t.Error("OutputRmsDbfs should not be nil after processing")
	}
}

func TestStreamConfig(t *testing.T) {
	sc, err := NewStreamConfig(48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if sc.FrameSize() != 480 {
		t.Errorf("FrameSize = %d, want 480", sc.FrameSize())
	}
	if sc.TotalSamples() != 960 {
		t.Errorf("TotalSamples = %d, want 960", sc.TotalSamples())
	}
}

func TestStreamConfigValidation(t *testing.T) {
	_, err := NewStreamConfig(0, 1)
	if err == nil {
		t.Error("expected error for sample rate 0")
	}

	_, err = NewStreamConfig(48000, 0)
	if err == nil {
		t.Error("expected error for 0 channels")
	}

	_, err = NewStreamConfig(48000, 9)
	if err == nil {
		t.Error("expected error for 9 channels")
	}
}

func TestApplyConfig(t *testing.T) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	cfg := DefaultConfig()
	hpf := DefaultHighPassFilterConfig()
	cfg.HighPassFilter = &hpf
	nsConfig := DefaultNsConfig()
	cfg.NoiseSuppression = &nsConfig

	err = ap.ApplyConfig(cfg)
	if err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	gotCfg := ap.Config()
	if !gotCfg.HighPassFilterEnabled() {
		t.Error("HPF should be enabled")
	}
	if !gotCfg.NoiseSuppressionEnabled() {
		t.Error("NS should be enabled")
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	ap, err := NewBuilder().
		SampleRate(16000).
		Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableNoiseSuppression(DefaultNsConfig()).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	defer ap.Close()

	frame := make([]float32, 160)
	for i := range frame {
		frame[i] = 0.01 * float32(math.Sin(2*math.Pi*440*float64(i)/16000))
	}
	data := [][]float32{frame}

	b.ResetTimer()
	for range b.N {
		ap.ProcessCaptureFloat(data)
	}
}
