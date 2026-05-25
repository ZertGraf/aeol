package rnn_vad

import "math"

// fft480 computes a 480-point real-to-complex DFT using the Goertzel algorithm.
// output is packed as [re[0], re[N/2], re[1], im[1], re[2], im[2], ...].
// Goertzel is O(N) per bin with zero precomputed storage, avoiding the 900KB
// lookup table that a naive DFT-matrix approach would require.
type fft480 struct {
	n int
}

func newFFT480() *fft480 {
	return &fft480{n: 480}
}

func (f *fft480) Close() {}

func (f *fft480) Forward(data []float32) {
	n := f.n
	tmp := make([]float32, n)
	copy(tmp, data)

	// DC (k=0): sum of all samples
	var dc float32
	for j := 0; j < n; j++ {
		dc += tmp[j]
	}
	data[0] = dc

	// Nyquist (k=N/2): alternating sum
	var nyq float32
	for j := 0; j < n; j++ {
		if j&1 == 0 {
			nyq += tmp[j]
		} else {
			nyq -= tmp[j]
		}
	}
	data[1] = nyq

	// bins k=1..N/2-1 via Goertzel
	for k := 1; k < n/2; k++ {
		w := 2.0 * math.Pi * float64(k) / float64(n)
		coeff := float32(2.0 * math.Cos(w))
		var s1, s2 float32
		for j := 0; j < n; j++ {
			s0 := tmp[j] + coeff*s1 - s2
			s2 = s1
			s1 = s0
		}
		cosW := float32(math.Cos(w))
		sinW := float32(math.Sin(w))
		data[2*k] = s1*cosW - s2
		data[2*k+1] = -(s1 * sinW)
	}
}
