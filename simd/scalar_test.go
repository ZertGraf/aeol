package simd

import (
	"math"
	"testing"
)

func TestDotProduct(t *testing.T) {
	b := &scalarBackend{}

	a := []float32{1, 2, 3, 4}
	c := []float32{5, 6, 7, 8}
	got := b.DotProduct(a, c)
	want := float32(1*5 + 2*6 + 3*7 + 4*8)
	if got != want {
		t.Errorf("DotProduct = %f, want %f", got, want)
	}
}

func TestDualDotProduct(t *testing.T) {
	b := &scalarBackend{}

	input := []float32{1, 2, 3}
	k1 := []float32{4, 5, 6}
	k2 := []float32{7, 8, 9}
	s1, s2 := b.DualDotProduct(input, k1, k2)

	want1 := float32(1*4 + 2*5 + 3*6)
	want2 := float32(1*7 + 2*8 + 3*9)
	if s1 != want1 || s2 != want2 {
		t.Errorf("DualDotProduct = (%f, %f), want (%f, %f)", s1, s2, want1, want2)
	}
}

func TestSum(t *testing.T) {
	b := &scalarBackend{}
	x := []float32{1, 2, 3, 4, 5}
	got := b.Sum(x)
	if got != 15 {
		t.Errorf("Sum = %f, want 15", got)
	}
}

func TestMultiplyAccumulate(t *testing.T) {
	b := &scalarBackend{}
	acc := []float32{0, 0, 0}
	a := []float32{1, 2, 3}
	c := []float32{4, 5, 6}
	b.MultiplyAccumulate(acc, a, c)

	want := []float32{4, 10, 18}
	for i := range acc {
		if acc[i] != want[i] {
			t.Errorf("acc[%d] = %f, want %f", i, acc[i], want[i])
		}
	}
}

func TestElementwiseSqrt(t *testing.T) {
	b := &scalarBackend{}
	x := []float32{4, 9, 16, 25}
	b.ElementwiseSqrt(x)
	want := []float32{2, 3, 4, 5}
	for i := range x {
		if math.Abs(float64(x[i]-want[i])) > 1e-6 {
			t.Errorf("x[%d] = %f, want %f", i, x[i], want[i])
		}
	}
}

func TestPowerSpectrum(t *testing.T) {
	b := &scalarBackend{}
	re := []float32{3, 0, 1}
	im := []float32{4, 1, 0}
	out := make([]float32, 3)
	b.PowerSpectrum(re, im, out)

	want := []float32{25, 1, 1}
	for i := range out {
		if out[i] != want[i] {
			t.Errorf("out[%d] = %f, want %f", i, out[i], want[i])
		}
	}
}

func TestConvolveSinc(t *testing.T) {
	b := &scalarBackend{}
	input := []float32{1, 1, 1, 1}
	k1 := []float64{0.25, 0.25, 0.25, 0.25}
	k2 := []float64{0.5, 0.5, 0, 0}
	got := b.ConvolveSinc(input, k1, k2, 0.0)
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("ConvolveSinc(factor=0) = %f, want 1.0", got)
	}
	got = b.ConvolveSinc(input, k1, k2, 1.0)
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("ConvolveSinc(factor=1) = %f, want 1.0", got)
	}
}

func BenchmarkDotProduct(b *testing.B) {
	backend := Default()
	a := make([]float32, 480)
	c := make([]float32, 480)
	for i := range a {
		a[i] = float32(i)
		c[i] = float32(i)
	}
	b.ResetTimer()
	for range b.N {
		backend.DotProduct(a, c)
	}
}

func BenchmarkPowerSpectrum(b *testing.B) {
	backend := Default()
	re := make([]float32, 129)
	im := make([]float32, 129)
	out := make([]float32, 129)
	b.ResetTimer()
	for range b.N {
		backend.PowerSpectrum(re, im, out)
	}
}
