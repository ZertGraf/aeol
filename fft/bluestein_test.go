package fft

import (
	"math"
	"testing"
)

func TestBluesteinMatchesOoura(t *testing.T) {
	for _, n := range []int{8, 16, 64, 128, 256, 512} {
		t.Run("", func(t *testing.T) {
			oo := NewOoura(n)
			bs := NewBluestein(n)

			dataO := make([]float32, n)
			dataB := make([]float32, n)
			for i := range dataO {
				dataO[i] = float32(math.Sin(2*math.Pi*float64(i)/float64(n)) +
					0.3*math.Cos(6*math.Pi*float64(i)/float64(n)))
				dataB[i] = dataO[i]
			}

			oo.Forward(dataO)
			bs.Forward(dataB)

			for i := range dataO {
				diff := math.Abs(float64(dataO[i] - dataB[i]))
				if diff > 0.05 {
					t.Errorf("n=%d bin %d: ooura=%f bluestein=%f diff=%f", n, i, dataO[i], dataB[i], diff)
				}
			}
		})
	}
}

func TestBluesteinRoundtrip(t *testing.T) {
	for _, n := range []int{10, 15, 30, 60, 120, 240, 480, 100, 300} {
		t.Run("", func(t *testing.T) {
			f := NewBluestein(n)
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
				diff := math.Abs(float64(data[i] - original[i]))
				if diff > 0.01 {
					t.Errorf("n=%d roundtrip mismatch at %d: got %f, want %f (diff=%f)",
						n, i, data[i], original[i], diff)
				}
			}
		})
	}
}

func TestBluestein480Tone(t *testing.T) {
	n := 480
	f := NewBluestein(n)
	data := make([]float32, n)

	// pure cosine at bin 5 (frequency = 5 * 24000/480 = 250 Hz)
	for i := range data {
		data[i] = float32(math.Cos(2 * math.Pi * 5 * float64(i) / float64(n)))
	}

	f.Forward(data)

	// bin 5 should have large magnitude; others small
	mag5 := math.Sqrt(float64(data[2*5])*float64(data[2*5]) + float64(data[2*5+1])*float64(data[2*5+1]))
	if mag5 < float64(n)/4 {
		t.Errorf("bin 5 magnitude = %f, expected ~%f", mag5, float64(n)/2)
	}

	for k := 0; k < n/2; k++ {
		if k == 5 {
			continue
		}
		var re, im float64
		if k == 0 {
			re = float64(data[0])
		} else {
			re = float64(data[2*k])
			im = float64(data[2*k+1])
		}
		mag := math.Sqrt(re*re + im*im)
		if mag > 1.0 {
			t.Errorf("bin %d magnitude = %f, expected ~0", k, mag)
		}
	}
}

func TestBluesteinParseval(t *testing.T) {
	for _, n := range []int{120, 240, 480} {
		t.Run("", func(t *testing.T) {
			f := NewBluestein(n)
			data := make([]float32, n)
			for i := range data {
				data[i] = float32(math.Sin(2*math.Pi*7*float64(i)/float64(n)) +
					0.5*math.Cos(2*math.Pi*13*float64(i)/float64(n)))
			}

			var timeEnergy float64
			for _, v := range data {
				timeEnergy += float64(v) * float64(v)
			}

			f.Forward(data)

			var freqEnergy float64
			freqEnergy += float64(data[0]) * float64(data[0])
			freqEnergy += float64(data[1]) * float64(data[1])
			for i := 2; i < n; i += 2 {
				freqEnergy += 2 * (float64(data[i])*float64(data[i]) + float64(data[i+1])*float64(data[i+1]))
			}
			freqEnergy /= float64(n)

			ratio := freqEnergy / timeEnergy
			if math.Abs(ratio-1.0) > 0.05 {
				t.Errorf("n=%d Parseval ratio = %f, want ~1.0", n, ratio)
			}
		})
	}
}

func TestDefaultFactoryArbitrary(t *testing.T) {
	f := DefaultFactory(480)
	if f.Size() != 480 {
		t.Errorf("DefaultFactory(480).Size() = %d, want 480", f.Size())
	}

	f2 := DefaultFactory(128)
	if f2.Size() != 128 {
		t.Errorf("DefaultFactory(128).Size() = %d, want 128", f2.Size())
	}
}

func BenchmarkBluestein480(b *testing.B) {
	f := NewBluestein(480)
	data := make([]float32, 480)
	for i := range data {
		data[i] = float32(math.Sin(float64(i) * 0.1))
	}
	b.ResetTimer()
	for range b.N {
		f.Forward(data)
	}
}
