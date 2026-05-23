package fft

import "math"

const (
	OouraFFTSize = 128
	OouraFFTHalf = OouraFFTSize / 2
)

type OouraFFT struct {
	n       int
	twRe    []float32
	twIm    []float32
	bitrev  []int
	tmpRe   [OouraFFTSize]float32
	tmpIm   [OouraFFTSize]float32
}

func NewOouraFFT() *OouraFFT {
	twRe, twIm := precomputeTwiddleF32(OouraFFTSize)
	return &OouraFFT{
		n:      OouraFFTSize,
		twRe:   twRe,
		twIm:   twIm,
		bitrev: precomputeBitrev(OouraFFTSize),
	}
}

func (o *OouraFFT) Forward(data []float32) {
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

func (o *OouraFFT) Inverse(data []float32) {
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

func (o *OouraFFT) ForwardSplit(data []float32, rOut, iOut []float32) {
	if len(data) < o.n {
		return
	}
	n := o.n
	re := o.tmpRe[:n]
	im := o.tmpIm[:n]
	copy(re, data[:n])
	clear(im)

	fftDIT(re, im, o.twRe, o.twIm, o.bitrev)

	rOut[0] = re[0]
	iOut[0] = 0
	rOut[n/2] = re[n/2]
	iOut[n/2] = 0
	for k := 1; k < n/2; k++ {
		rOut[k] = re[k]
		iOut[k] = im[k]
	}
}

func (o *OouraFFT) InverseSplit(rIn, iIn []float32, data []float32) {
	n := o.n
	data[0] = rIn[0]
	data[1] = rIn[n/2]
	for k := 1; k < n/2; k++ {
		data[2*k] = rIn[k]
		data[2*k+1] = iIn[k]
	}
	o.Inverse(data)
}

type FFT4G struct {
	n      int
	twRe   []float32
	twIm   []float32
	bitrev []int
	tmpRe  []float32
	tmpIm  []float32
}

func NewFFT4G(n int) *FFT4G {
	if n < 4 || n&(n-1) != 0 {
		panic("fft: size must be a power of 2 >= 4")
	}
	twRe, twIm := precomputeTwiddleF32(n)
	return &FFT4G{
		n:      n,
		twRe:   twRe,
		twIm:   twIm,
		bitrev: precomputeBitrev(n),
		tmpRe:  make([]float32, n),
		tmpIm:  make([]float32, n),
	}
}

func (f *FFT4G) Forward(data []float32) {
	n := f.n
	re := f.tmpRe[:n]
	im := f.tmpIm[:n]
	copy(re, data[:n])
	clear(im)

	fftDIT(re, im, f.twRe, f.twIm, f.bitrev)
	data[0] = re[0]
	data[1] = re[n/2]
	for k := 1; k < n/2; k++ {
		data[2*k] = re[k]
		data[2*k+1] = im[k]
	}
}

func (f *FFT4G) Inverse(data []float32) {
	n := f.n
	re := f.tmpRe[:n]
	im := f.tmpIm[:n]
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

	ifftDIT(re, im, f.twRe, f.twIm, f.bitrev)
	copy(data[:n], re)
}

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
