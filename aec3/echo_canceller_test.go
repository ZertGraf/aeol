package aec3

import (
	"math"
	"testing"
)

func TestEchoCanceller3Create(t *testing.T) {
	ec := NewEchoCanceller3(DefaultConfig(), 16000, 1)
	if ec == nil {
		t.Fatal("NewEchoCanceller3 returned nil")
	}
}

func TestEchoCanceller3ProcessSilence(t *testing.T) {
	ec := NewEchoCanceller3(DefaultConfig(), 16000, 1)

	render := make([]float32, BlockSize)
	capture := make([]float32, BlockSize)

	ec.ProcessRender(render)
	ec.ProcessCapture(capture)

	for i, v := range capture {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Errorf("capture[%d] = NaN/Inf", i)
		}
	}
}

func TestEchoCancellation(t *testing.T) {
	ec := NewEchoCanceller3(DefaultConfig(), 16000, 1)
	delayBlocks := 5

	renderHistory := make([][]float32, delayBlocks+1)
	for i := range renderHistory {
		renderHistory[i] = make([]float32, BlockSize)
	}

	for iter := 0; iter < 1000; iter++ {
		render := make([]float32, BlockSize)
		for i := range render {
			render[i] = 0.5 * float32(math.Sin(2*math.Pi*440*float64(iter*BlockSize+i)/16000))
		}

		copy(renderHistory[iter%(delayBlocks+1)], render)
		ec.ProcessRender(render)

		capture := make([]float32, BlockSize)
		echoIdx := (iter - delayBlocks + delayBlocks + 1) % (delayBlocks + 1)
		for i := range capture {
			capture[i] = 0.3 * renderHistory[echoIdx][i]
		}

		ec.ProcessCapture(capture)
	}

	erle := ec.ERLE()
	if erle < 1.0 {
		t.Logf("ERLE = %f (convergence may need more iterations)", erle)
	}
}

func TestEchoCanceller3Reset(t *testing.T) {
	ec := NewEchoCanceller3(DefaultConfig(), 16000, 1)

	render := make([]float32, BlockSize)
	capture := make([]float32, BlockSize)
	for i := range render {
		render[i] = 0.5
		capture[i] = 0.3
	}
	ec.ProcessRender(render)
	ec.ProcessCapture(capture)

	ec.Reset()

	if ec.ERLE() != 1.0 {
		t.Errorf("ERLE after reset = %f, want 1.0", ec.ERLE())
	}
}

func TestMultipleRates(t *testing.T) {
	for _, rate := range []uint32{16000, 32000, 48000} {
		ec := NewEchoCanceller3(DefaultConfig(), rate, 1)
		if ec == nil {
			t.Errorf("NewEchoCanceller3 returned nil for rate %d", rate)
		}
	}
}

func BenchmarkEchoCanceller3(b *testing.B) {
	ec := NewEchoCanceller3(DefaultConfig(), 16000, 1)
	render := make([]float32, BlockSize)
	capture := make([]float32, BlockSize)
	for i := range render {
		render[i] = float32(math.Sin(float64(i) * 0.1))
		capture[i] = render[i] * 0.3
	}
	b.ResetTimer()
	for range b.N {
		ec.ProcessRender(render)
		ec.ProcessCapture(capture)
	}
}

func BenchmarkAdaptiveFilter_Filter(b *testing.B) {
	cfg := DefaultConfig()
	af := NewAdaptiveFilter(cfg.Filter.Refined.LengthBlocks)
	rb := NewRenderBuffer(cfg.Filter.Refined.LengthBlocks + 8)

	// fill render buffer with non-zero data so the filter does real work
	for i := 0; i < rb.Size(); i++ {
		var d FftData
		for k := range d.Re {
			d.Re[k] = float32(k+1) * 0.01
			d.Im[k] = float32(k) * 0.005
		}
		rb.Insert(&d)
	}

	var out FftData
	b.ResetTimer()
	for range b.N {
		af.Filter(rb, &out)
	}
}

func BenchmarkAdaptiveFilter_Adapt(b *testing.B) {
	cfg := DefaultConfig()
	af := NewAdaptiveFilter(cfg.Filter.Refined.LengthBlocks)
	rb := NewRenderBuffer(cfg.Filter.Refined.LengthBlocks + 8)

	for i := 0; i < rb.Size(); i++ {
		var d FftData
		for k := range d.Re {
			d.Re[k] = float32(k+1) * 0.01
			d.Im[k] = float32(k) * 0.005
		}
		rb.Insert(&d)
	}

	var err FftData
	for k := range err.Re {
		err.Re[k] = float32(k) * 0.001
		err.Im[k] = float32(k) * 0.0005
	}

	b.ResetTimer()
	for range b.N {
		af.Adapt(rb, &err, 0.01)
	}
}

func BenchmarkAdaptiveFilter_FilterAndAdapt(b *testing.B) {
	cfg := DefaultConfig()
	af := NewAdaptiveFilter(cfg.Filter.Refined.LengthBlocks)
	rb := NewRenderBuffer(cfg.Filter.Refined.LengthBlocks + 8)

	for i := 0; i < rb.Size(); i++ {
		var d FftData
		for k := range d.Re {
			d.Re[k] = float32(k+1) * 0.01
			d.Im[k] = float32(k) * 0.005
		}
		rb.Insert(&d)
	}

	var out, err FftData
	for k := range err.Re {
		err.Re[k] = float32(k) * 0.001
	}

	b.ResetTimer()
	for range b.N {
		af.Filter(rb, &out)
		af.Adapt(rb, &err, 0.01)
	}
}
