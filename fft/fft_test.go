package fft

import (
	"math"
	"testing"
)

func TestOouraRoundtrip(t *testing.T) {
	o := NewOouraFFT()

	original := make([]float32, OouraFFTSize)
	data := make([]float32, OouraFFTSize)
	for i := range data {
		data[i] = float32(math.Sin(2 * math.Pi * float64(i) / float64(OouraFFTSize)))
		original[i] = data[i]
	}

	o.Forward(data)
	o.Inverse(data)

	for i := range data {
		if math.Abs(float64(data[i]-original[i])) > 1e-4 {
			t.Errorf("roundtrip mismatch at %d: got %f, want %f", i, data[i], original[i])
		}
	}
}

func TestOouraSplitRoundtrip(t *testing.T) {
	o := NewOouraFFT()

	original := make([]float32, OouraFFTSize)
	data := make([]float32, OouraFFTSize)
	re := make([]float32, OouraFFTSize/2+1)
	im := make([]float32, OouraFFTSize/2+1)

	for i := range data {
		data[i] = float32(math.Cos(2 * math.Pi * 3 * float64(i) / float64(OouraFFTSize)))
		original[i] = data[i]
	}

	o.ForwardSplit(data, re, im)

	data2 := make([]float32, OouraFFTSize)
	o.InverseSplit(re, im, data2)

	for i := range data2 {
		if math.Abs(float64(data2[i]-original[i])) > 1e-4 {
			t.Errorf("split roundtrip mismatch at %d: got %f, want %f", i, data2[i], original[i])
		}
	}
}

func TestFFT4GRoundtrip(t *testing.T) {
	for _, n := range []int{4, 8, 16, 32, 64, 128, 256, 512} {
		t.Run("", func(t *testing.T) {
			f := NewFFT4G(n)
			original := make([]float32, n)
			data := make([]float32, n)
			for i := range data {
				data[i] = float32(math.Sin(2*math.Pi*float64(i)/float64(n)) +
					0.5*math.Cos(4*math.Pi*float64(i)/float64(n)))
				original[i] = data[i]
			}

			f.Forward(data)
			f.Inverse(data)

			for i := range data {
				if math.Abs(float64(data[i]-original[i])) > 1e-3 {
					t.Errorf("n=%d roundtrip mismatch at %d: got %f, want %f", n, i, data[i], original[i])
				}
			}
		})
	}
}

func TestParsevalEnergy(t *testing.T) {
	o := NewOouraFFT()
	data := make([]float32, OouraFFTSize)
	for i := range data {
		data[i] = float32(math.Sin(2 * math.Pi * 5 * float64(i) / float64(OouraFFTSize)))
	}

	var timeEnergy float64
	for _, v := range data {
		timeEnergy += float64(v) * float64(v)
	}

	o.Forward(data)

	var freqEnergy float64
	freqEnergy += float64(data[0]) * float64(data[0])
	freqEnergy += float64(data[1]) * float64(data[1])
	for i := 2; i < OouraFFTSize; i += 2 {
		freqEnergy += 2 * (float64(data[i])*float64(data[i]) + float64(data[i+1])*float64(data[i+1]))
	}
	freqEnergy /= float64(OouraFFTSize)

	ratio := freqEnergy / timeEnergy
	if math.Abs(ratio-1.0) > 0.05 {
		t.Errorf("Parseval energy ratio = %f, want ~1.0", ratio)
	}
}

func BenchmarkOouraFFTForward(b *testing.B) {
	o := NewOouraFFT()
	data := make([]float32, OouraFFTSize)
	for i := range data {
		data[i] = float32(i)
	}
	b.ResetTimer()
	for range b.N {
		o.Forward(data)
	}
}
