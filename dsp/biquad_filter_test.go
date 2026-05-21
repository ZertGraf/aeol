package dsp

import (
	"math"
	"testing"
)

func TestBiQuadFilterLowPass(t *testing.T) {
	coeffs := BiQuadCoefficients{
		B: [3]float32{0.0675, 0.1349, 0.0675},
		A: [2]float32{-1.1430, 0.4128},
	}
	f := NewBiQuadFilter(coeffs)

	in := make([]float32, 160)
	for i := range in {
		in[i] = float32(math.Sin(2 * math.Pi * 7000 * float64(i) / 16000))
	}

	out := make([]float32, 160)
	f.Process(in, out)

	var inPower, outPower float64
	for i := 80; i < 160; i++ {
		inPower += float64(in[i]) * float64(in[i])
		outPower += float64(out[i]) * float64(out[i])
	}

	if outPower > inPower {
		t.Errorf("high freq not attenuated: in=%f, out=%f", inPower, outPower)
	}
}

func TestCascadedBiQuadFilterReset(t *testing.T) {
	coeffs := []BiQuadCoefficients{
		{B: [3]float32{1, 0, 0}, A: [2]float32{0, 0}},
	}
	f := NewCascadedBiQuadFilter(coeffs)

	in := []float32{1, 2, 3}
	out := make([]float32, 3)
	f.Process(in, out)

	f.Reset()

	out2 := make([]float32, 3)
	f.Process(in, out2)
	for i := range out {
		if out[i] != out2[i] {
			t.Errorf("after reset: out[%d]=%f, out2[%d]=%f", i, out[i], i, out2[i])
		}
	}
}

func TestDownmixToMono(t *testing.T) {
	channels := [][]float32{
		{1, 2, 3},
		{5, 6, 7},
	}
	mono := make([]float32, 3)
	DownmixToMono(channels, mono)

	want := []float32{3, 4, 5}
	for i := range mono {
		if math.Abs(float64(mono[i]-want[i])) > 1e-6 {
			t.Errorf("mono[%d]=%f, want %f", i, mono[i], want[i])
		}
	}
}

func TestDeinterleave(t *testing.T) {
	interleaved := []float32{1, 2, 3, 4, 5, 6}
	out := make([][]float32, 2)
	out[0] = make([]float32, 3)
	out[1] = make([]float32, 3)

	Deinterleave(interleaved, 2, out)

	wantL := []float32{1, 3, 5}
	wantR := []float32{2, 4, 6}
	for i := range wantL {
		if out[0][i] != wantL[i] {
			t.Errorf("L[%d]=%f, want %f", i, out[0][i], wantL[i])
		}
		if out[1][i] != wantR[i] {
			t.Errorf("R[%d]=%f, want %f", i, out[1][i], wantR[i])
		}
	}
}

func TestInterleave(t *testing.T) {
	planar := [][]float32{{1, 3, 5}, {2, 4, 6}}
	out := make([]float32, 6)
	Interleave(planar, 2, out)
	want := []float32{1, 2, 3, 4, 5, 6}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d]=%f, want %f", i, out[i], want[i])
		}
	}
}

func TestRmsLevel(t *testing.T) {
	samples := make([]float32, 160)
	for i := range samples {
		samples[i] = 0.5
	}
	rms := RmsLevel(samples)
	if math.Abs(float64(rms)-0.5) > 0.01 {
		t.Errorf("RmsLevel = %f, want ~0.5", rms)
	}
}
