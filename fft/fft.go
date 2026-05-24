// Package fft defines the FFT interface and provides a pure Go backend (Ooura).
// Processing stages accept an fft.Factory to allow backend selection at construction time.
// The default backend requires no CGO; for SIMD acceleration see fft/pffft.
package fft

import "math"

// FFT defines the interface for FFT backends. All methods use packed format:
// after Forward, data contains [re[0], re[N/2], re[1], im[1], re[2], im[2], ...].
// Inverse expects the same layout. Not safe for concurrent use.
type FFT interface {
	Forward(data []float32)
	Inverse(data []float32)
	Size() int
}

// Factory creates an FFT of the given size. Used by processing stages
// to let the caller choose the FFT backend.
type Factory func(size int) FFT

// DefaultFactory creates a pure Go Ooura FFT. Zero CGO dependencies.
func DefaultFactory(size int) FFT {
	return NewOoura(size)
}

// ForwardSplit runs Forward then unpacks to separate re/im arrays.
// rOut and iOut must have length >= Size()/2+1.
func ForwardSplit(f FFT, data []float32, rOut, iOut []float32) {
	f.Forward(data)
	n := f.Size()
	rOut[0] = data[0]
	iOut[0] = 0
	rOut[n/2] = data[1]
	iOut[n/2] = 0
	for k := 1; k < n/2; k++ {
		rOut[k] = data[2*k]
		iOut[k] = data[2*k+1]
	}
}

// InverseSplit packs separate re/im arrays then runs Inverse.
// rIn and iIn must have length >= Size()/2+1.
func InverseSplit(f FFT, rIn, iIn []float32, data []float32) {
	n := f.Size()
	data[0] = rIn[0]
	data[1] = rIn[n/2]
	for k := 1; k < n/2; k++ {
		data[2*k] = rIn[k]
		data[2*k+1] = iIn[k]
	}
	f.Inverse(data)
}

const (
	// OouraFFTSize is the default FFT size used by AEC3 (128 points).
	OouraFFTSize = 128
	// OouraFFTHalf is N/2 — half the FFT size. the number of unique complex bins
	// for a real FFT is N/2+1 (see aec3.FFTSizeBy2Plus1).
	OouraFFTHalf = OouraFFTSize / 2
)

type ooura struct {
	n      int
	twRe   []float32
	twIm   []float32
	bitrev []int
	tmpRe  []float32
	tmpIm  []float32
}

// NewOoura creates a pure Go radix-2 DIT FFT of the given size (must be a power of 2, >= 4).
// twiddle factors and bit-reversal table are precomputed at construction time.
// forward and inverse operations use the packed format described in the FFT interface.
func NewOoura(size int) *ooura {
	if size < 4 || size&(size-1) != 0 {
		panic("fft: size must be a power of 2 >= 4")
	}
	twRe, twIm := precomputeTwiddleF32(size)
	return &ooura{
		n:      size,
		twRe:   twRe,
		twIm:   twIm,
		bitrev: precomputeBitrev(size),
		tmpRe:  make([]float32, size),
		tmpIm:  make([]float32, size),
	}
}

func (o *ooura) Size() int { return o.n }

func (o *ooura) Forward(data []float32) {
	if len(data) < o.n {
		return
	}
	n := o.n
	re := o.tmpRe[:n]
	im := o.tmpIm[:n]
	copy(re, data[:n])
	clear(im)

	fftDIT(re, im, o.twRe, o.twIm, o.bitrev)
	data[0] = re[0]
	data[1] = re[n/2]
	for k := 1; k < n/2; k++ {
		data[2*k] = re[k]
		data[2*k+1] = im[k]
	}
}

func (o *ooura) Inverse(data []float32) {
	if len(data) < o.n {
		return
	}
	n := o.n
	re := o.tmpRe[:n]
	im := o.tmpIm[:n]
	clear(re)
	clear(im)

	re[0] = data[0]
	re[n/2] = data[1]
	for k := 1; k < n/2; k++ {
		re[k] = data[2*k]
		im[k] = data[2*k+1]
		re[n-k] = data[2*k]
		im[n-k] = -data[2*k+1]
	}

	ifftDIT(re, im, o.twRe, o.twIm, o.bitrev)
	copy(data[:n], re)
}

// NewOouraFFT creates a 128-point pure Go FFT. Kept for backward compatibility.
func NewOouraFFT() *ooura { return NewOoura(OouraFFTSize) }

// NewFFT4G creates a generic-size pure Go FFT. Kept for backward compatibility.
func NewFFT4G(n int) *ooura { return NewOoura(n) }

// OouraFFT is an alias for backward compatibility.
type OouraFFT = ooura

// FFT4G is an alias for backward compatibility.
type FFT4G = ooura

func precomputeTwiddleF32(n int) ([]float32, []float32) {
	half := n / 2
	twRe := make([]float32, half)
	twIm := make([]float32, half)
	for k := 0; k < half; k++ {
		angle := -2 * math.Pi * float64(k) / float64(n)
		twRe[k] = float32(math.Cos(angle))
		twIm[k] = float32(math.Sin(angle))
	}
	return twRe, twIm
}

func precomputeBitrev(n int) []int {
	var pairs []int
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			pairs = append(pairs, i, j)
		}
		k := n >> 1
		for k <= j {
			j -= k
			k >>= 1
		}
		j += k
	}
	return pairs
}

func fftDIT(re, im []float32, twRe, twIm []float32, bitrev []int) {
	n := len(re)

	for i := 0; i < len(bitrev); i += 2 {
		a, b := bitrev[i], bitrev[i+1]
		re[a], re[b] = re[b], re[a]
		im[a], im[b] = im[b], im[a]
	}

	_ = re[n-1]
	_ = im[n-1]

	for size := 2; size <= n; size *= 2 {
		half := size / 2
		step := n / size
		twR := twRe[:half*step]
		twI := twIm[:half*step]
		for j := 0; j < n; j += size {
			r1 := re[j : j+size : j+size]
			m1 := im[j : j+size : j+size]
			for k := 0; k < half; k++ {
				idx := k * step
				twr := twR[idx]
				twi := twI[idx]
				r2k := r1[k+half]
				m2k := m1[k+half]
				tr := twr*r2k - twi*m2k
				ti := twr*m2k + twi*r2k
				r1[k+half] = r1[k] - tr
				m1[k+half] = m1[k] - ti
				r1[k] += tr
				m1[k] += ti
			}
		}
	}
}

func ifftDIT(re, im []float32, twRe, twIm []float32, bitrev []int) {
	n := len(re)
	for i := 0; i < n; i++ {
		im[i] = -im[i]
	}
	fftDIT(re, im, twRe, twIm, bitrev)
	s := 1.0 / float32(n)
	for i := 0; i < n; i++ {
		re[i] *= s
		im[i] = -im[i] * s
	}
}
