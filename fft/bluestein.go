package fft

import "math"

// bluestein implements an arbitrary-size real FFT using the Bluestein (chirp-z)
// algorithm. it converts the N-point DFT into a circular convolution of length M
// (next power of 2 >= 2N-1) and uses the existing radix-2 DIT FFT internally.
// output format is identical to ooura: [re[0], re[N/2], re[1], im[1], ...].
type bluestein struct {
	n int // original transform size
	m int // padded power-of-2 size for convolution

	// chirp[k] = exp(-jπk²/N): modulation/demodulation factors
	chirpRe []float32 // cos(πk²/N), length N
	chirpIm []float32 // -sin(πk²/N), length N

	// precomputed FFT of the chirp convolution kernel, length M
	bRe []float32
	bIm []float32

	// twiddle factors and bit-reversal for the M-point complex FFT
	twRe   []float32
	twIm   []float32
	bitrev []int

	// work buffers, length M each
	aRe, aIm []float32
	cRe, cIm []float32
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// NewBluestein creates an FFT for arbitrary size N >= 2 using the chirp-z algorithm.
func NewBluestein(n int) *bluestein {
	if n < 2 {
		panic("fft: bluestein size must be >= 2")
	}

	m := nextPow2(2*n - 1)
	twRe, twIm := precomputeTwiddleF32(m)
	bitrev := precomputeBitrev(m)

	chirpRe := make([]float32, n)
	chirpIm := make([]float32, n)
	for k := 0; k < n; k++ {
		// chirp[k] = exp(-jπk²/N)
		angle := math.Pi * float64(k) * float64(k) / float64(n)
		chirpRe[k] = float32(math.Cos(angle))
		chirpIm[k] = float32(-math.Sin(angle))
	}

	// build the chirp convolution kernel h_circ in frequency domain
	// h[k] = conj(chirp[k]) = exp(+jπk²/N)
	hRe := make([]float32, m)
	hIm := make([]float32, m)
	hRe[0] = 1 // h[0] = 1
	for k := 1; k < n; k++ {
		hRe[k] = chirpRe[k]       // cos(πk²/N)
		hIm[k] = -chirpIm[k]      // sin(πk²/N)
		hRe[m-k] = chirpRe[k]     // wrap: h[-k] = h[k]
		hIm[m-k] = -chirpIm[k]
	}

	fftDIT(hRe, hIm, twRe, twIm, bitrev)

	return &bluestein{
		n: n, m: m,
		chirpRe: chirpRe, chirpIm: chirpIm,
		bRe: hRe, bIm: hIm,
		twRe: twRe, twIm: twIm, bitrev: bitrev,
		aRe: make([]float32, m), aIm: make([]float32, m),
		cRe: make([]float32, m), cIm: make([]float32, m),
	}
}

func (b *bluestein) Size() int { return b.n }

func (b *bluestein) Forward(data []float32) {
	n, m := b.n, b.m

	// modulate: a[k] = x[k] * chirp[k], zero-padded to M
	clear(b.aRe)
	clear(b.aIm)
	for k := 0; k < n; k++ {
		x := data[k]
		b.aRe[k] = x * b.chirpRe[k]
		b.aIm[k] = x * b.chirpIm[k]
	}

	// convolution in frequency domain: A = FFT(a), C = A * B, c = IFFT(C)
	// ifftDIT handles 1/M scaling, so no extra normalization here
	fftDIT(b.aRe, b.aIm, b.twRe, b.twIm, b.bitrev)

	for k := 0; k < m; k++ {
		ar, ai := b.aRe[k], b.aIm[k]
		br, bi := b.bRe[k], b.bIm[k]
		b.cRe[k] = ar*br - ai*bi
		b.cIm[k] = ar*bi + ai*br
	}

	ifftDIT(b.cRe, b.cIm, b.twRe, b.twIm, b.bitrev)

	// demodulate: X[k] = chirp[k] * c[k]
	// pack into standard format: [re[0], re[N/2], re[1], im[1], ...]
	xr0 := b.chirpRe[0]*b.cRe[0] - b.chirpIm[0]*b.cIm[0]
	data[0] = xr0

	half := n / 2
	xrH := b.chirpRe[half]*b.cRe[half] - b.chirpIm[half]*b.cIm[half]
	data[1] = xrH

	for k := 1; k < half; k++ {
		cr, ci := b.chirpRe[k], b.chirpIm[k]
		dr, di := b.cRe[k], b.cIm[k]
		data[2*k] = cr*dr - ci*di
		data[2*k+1] = cr*di + ci*dr
	}
}

func (b *bluestein) Inverse(data []float32) {
	n, m := b.n, b.m
	half := n / 2

	// reconstruct full complex spectrum X[0..N-1] from packed format
	xRe := b.cRe[:n]
	xIm := b.cIm[:n]
	xRe[0] = data[0]
	xIm[0] = 0
	xRe[half] = data[1]
	xIm[half] = 0
	for k := 1; k < half; k++ {
		xRe[k] = data[2*k]
		xIm[k] = data[2*k+1]
		xRe[n-k] = data[2*k]
		xIm[n-k] = -data[2*k+1]
	}

	// inverse DFT via Bluestein: x[n] = (1/N) * conj(w[n]) * Σ (X[k]*conj(w[k])) * w[k-n]
	// modulate: a[k] = X[k] * conj(chirp[k])
	clear(b.aRe[:m])
	clear(b.aIm[:m])
	for k := 0; k < n; k++ {
		cr, ci := b.chirpRe[k], -b.chirpIm[k]
		b.aRe[k] = xRe[k]*cr - xIm[k]*ci
		b.aIm[k] = xRe[k]*ci + xIm[k]*cr
	}

	fftDIT(b.aRe, b.aIm, b.twRe, b.twIm, b.bitrev)

	// inverse kernel: B_inv[k] = conj(B[(M-k) mod M])
	for k := 0; k < m; k++ {
		mk := (m - k) % m
		br := b.bRe[mk]
		bi := -b.bIm[mk]
		ar, ai := b.aRe[k], b.aIm[k]
		b.aRe[k] = ar*br - ai*bi
		b.aIm[k] = ar*bi + ai*br
	}

	ifftDIT(b.aRe, b.aIm, b.twRe, b.twIm, b.bitrev)

	// demodulate: x[n] = (1/N) * conj(chirp[n]) * c[n]
	invN := 1.0 / float32(n)
	for k := 0; k < n; k++ {
		cr := b.chirpRe[k]
		ci := -b.chirpIm[k] // conj(chirp)
		data[k] = (cr*b.aRe[k] - ci*b.aIm[k]) * invN
	}
}
