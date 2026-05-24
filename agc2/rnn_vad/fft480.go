package rnn_vad

import "math"

// fft480 implements a pure Go, precomputed 480-point real-to-complex DFT.
// It avoids the CGO dependency of pffft while being fast enough for real-time
// (uses a precomputed 900KB table, ~0.1ms per frame).
type fft480 struct {
	cosTab []float32
	sinTab []float32
	n      int
}

func newFFT480() *fft480 {
	n := 480
	f := &fft480{
		n:      n,
		cosTab: make([]float32, n*n),
		sinTab: make([]float32, n*n),
	}
	for k := 0; k < n; k++ {
		for j := 0; j < n; j++ {
			f.cosTab[k*n+j] = float32(math.Cos(-2 * math.Pi * float64(k) * float64(j) / float64(n)))
			f.sinTab[k*n+j] = float32(math.Sin(-2 * math.Pi * float64(k) * float64(j) / float64(n)))
		}
	}
	return f
}

func (f *fft480) Close() {}

// Forward computes the DFT and packs it as: [re[0], re[N/2], re[1], im[1], ...]
func (f *fft480) Forward(data []float32) {
	n := f.n
	tmp := make([]float32, n)
	copy(tmp, data)

	// k = 0
	var sumRe float32
	for j := 0; j < n; j++ {
		sumRe += tmp[j] // cos(0)=1
	}
	data[0] = sumRe

	// k = N/2
	var sumReNyq float32
	for j := 0; j < n; j++ {
		sumReNyq += tmp[j] * f.cosTab[(n/2)*n+j]
	}
	data[1] = sumReNyq

	// k = 1..N/2-1
	for k := 1; k < n/2; k++ {
		var r, i float32
		idx := k * n
		for j := 0; j < n; j++ {
			r += tmp[j] * f.cosTab[idx+j]
			i += tmp[j] * f.sinTab[idx+j]
		}
		data[2*k] = r
		data[2*k+1] = i
	}
}
