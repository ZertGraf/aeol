// Package fft provides Fast Fourier Transform implementations for audio processing.
package fft

import "math"

const (
	OouraFFTSize = 128
	OouraFFTHalf = OouraFFTSize / 2
)

type OouraFFT struct {
	n       int
	twiddle []complex128
}

func NewOouraFFT() *OouraFFT {
	return &OouraFFT{
		n:       OouraFFTSize,
		twiddle: precomputeTwiddle(OouraFFTSize),
	}
}

// Forward computes the real FFT of data in-place.
// Output format: data[0]=DC, data[1]=Nyquist, data[2k]=Re[k], data[2k+1]=Im[k]
func (o *OouraFFT) Forward(data []float32) {
	if len(data) < o.n {
		return
	}
	re, im := realToComplex(data[:o.n])
	fftDIT(re, im, o.twiddle)
	data[0] = re[0]
	data[1] = re[o.n/2]
	for k := 1; k < o.n/2; k++ {
		data[2*k] = re[k]
		data[2*k+1] = im[k]
	}
}

// Inverse computes the inverse real FFT in-place.
func (o *OouraFFT) Inverse(data []float32) {
	if len(data) < o.n {
		return
	}
	n := o.n
	re := make([]float32, n)
	im := make([]float32, n)

	re[0] = data[0]
	re[n/2] = data[1]
	for k := 1; k < n/2; k++ {
		re[k] = data[2*k]
		im[k] = data[2*k+1]
		re[n-k] = data[2*k]
		im[n-k] = -data[2*k+1]
	}

	ifftDIT(re, im, o.twiddle)
	copy(data[:n], re)
}

func (o *OouraFFT) ForwardSplit(data []float32, rOut, iOut []float32) {
	o.Forward(data)
	n := o.n
	rOut[0] = data[0]
	iOut[0] = 0
	rOut[n/2] = data[1]
	iOut[n/2] = 0
	for k := 1; k < n/2; k++ {
		rOut[k] = data[2*k]
		iOut[k] = data[2*k+1]
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
	n       int
	twiddle []complex128
}

func NewFFT4G(n int) *FFT4G {
	if n < 4 || n&(n-1) != 0 {
		panic("fft: size must be a power of 2 >= 4")
	}
	return &FFT4G{
		n:       n,
		twiddle: precomputeTwiddle(n),
	}
}

func (f *FFT4G) Forward(data []float32) {
	re, im := realToComplex(data[:f.n])
	fftDIT(re, im, f.twiddle)
	data[0] = re[0]
	data[1] = re[f.n/2]
	for k := 1; k < f.n/2; k++ {
		data[2*k] = re[k]
		data[2*k+1] = im[k]
	}
}

func (f *FFT4G) Inverse(data []float32) {
	n := f.n
	re := make([]float32, n)
	im := make([]float32, n)

	re[0] = data[0]
	re[n/2] = data[1]
	for k := 1; k < n/2; k++ {
		re[k] = data[2*k]
		im[k] = data[2*k+1]
		re[n-k] = data[2*k]
		im[n-k] = -data[2*k+1]
	}

	ifftDIT(re, im, f.twiddle)
	copy(data[:n], re)
}

func precomputeTwiddle(n int) []complex128 {
	tw := make([]complex128, n/2)
	for k := range tw {
		angle := -2 * math.Pi * float64(k) / float64(n)
		tw[k] = complex(math.Cos(angle), math.Sin(angle))
	}
	return tw
}

func realToComplex(data []float32) ([]float32, []float32) {
	n := len(data)
	re := make([]float32, n)
	im := make([]float32, n)
	copy(re, data)
	return re, im
}

func fftDIT(re, im []float32, twiddle []complex128) {
	n := len(re)
	bitReverseSwap(re, im, n)

	for size := 2; size <= n; size *= 2 {
		half := size / 2
		step := n / size
		for j := 0; j < n; j += size {
			for k := 0; k < half; k++ {
				tw := twiddle[k*step]
				twr := float32(real(tw))
				twi := float32(imag(tw))
				i1 := j + k
				i2 := j + k + half
				tr := twr*re[i2] - twi*im[i2]
				ti := twr*im[i2] + twi*re[i2]
				re[i2] = re[i1] - tr
				im[i2] = im[i1] - ti
				re[i1] += tr
				im[i1] += ti
			}
		}
	}
}

func ifftDIT(re, im []float32, twiddle []complex128) {
	n := len(re)
	for i := range im[:n] {
		im[i] = -im[i]
	}
	fftDIT(re, im, twiddle)
	s := 1.0 / float32(n)
	for i := range re[:n] {
		re[i] *= s
		im[i] = -im[i] * s
	}
}

func bitReverseSwap(re, im []float32, n int) {
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
		k := n >> 1
		for k <= j {
			j -= k
			k >>= 1
		}
		j += k
	}
}
