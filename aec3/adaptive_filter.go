package aec3

// AdaptiveFilter is a frequency-domain NLMS filter that models the echo path.
// coefficients are stored as complex spectra, one FftData per filter block.
type AdaptiveFilter struct {
	filterLength int
	coeffs       []FftData
}

// NewAdaptiveFilter allocates an AdaptiveFilter with lengthBlocks complex-spectrum coefficient blocks.
func NewAdaptiveFilter(lengthBlocks int) *AdaptiveFilter {
	return &AdaptiveFilter{
		filterLength: lengthBlocks,
		coeffs:       make([]FftData, lengthBlocks),
	}
}

// Filter computes the frequency-domain echo estimate by convolving the filter coefficients
// with the render spectra from renderBuffer, writing the result into output.
func (af *AdaptiveFilter) Filter(renderBuffer *RenderBuffer, output *FftData) {
	output.Clear()
	outRe := &output.Re
	outIm := &output.Im
	for i := 0; i < af.filterLength; i++ {
		c := &af.coeffs[i]
		r := renderBuffer.Block(i)
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			outRe[k] += c.Re[k]*r.Re[k] - c.Im[k]*r.Im[k]
			outIm[k] += c.Re[k]*r.Im[k] + c.Im[k]*r.Re[k]
		}
	}
}

// Adapt updates the filter coefficients using the NLMS gradient step:
// coeffs += stepSize * conj(render) * error, applied per frequency bin.
func (af *AdaptiveFilter) Adapt(renderBuffer *RenderBuffer, errSignal *FftData, stepSize float32) {
	eRe := &errSignal.Re
	eIm := &errSignal.Im
	for i := 0; i < af.filterLength; i++ {
		c := &af.coeffs[i]
		r := renderBuffer.Block(i)
		rRe := &r.Re
		rIm := &r.Im
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			er := eRe[k]
			ei := eIm[k]
			c.Re[k] += stepSize * (er*rRe[k] + ei*rIm[k])
			c.Im[k] += stepSize * (-er*rIm[k] + ei*rRe[k])
		}
	}
}

// Energy returns the sum of squared magnitudes across all coefficient bins.
// useful for detecting filter divergence.
func (af *AdaptiveFilter) Energy() float32 {
	var energy float32
	for i := range af.coeffs {
		c := &af.coeffs[i]
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			energy += c.Re[k]*c.Re[k] + c.Im[k]*c.Im[k]
		}
	}
	return energy
}

// ScaleFilter multiplies all coefficient bins by scale.
// used for misadjustment correction when the refined filter diverges.
func (af *AdaptiveFilter) ScaleFilter(scale float32) {
	for i := range af.coeffs {
		c := &af.coeffs[i]
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			c.Re[k] *= scale
			c.Im[k] *= scale
		}
	}
}

// CopyFrom copies coefficient blocks from other into af.
// if lengths differ, the shorter is used and remaining blocks in af are cleared.
func (af *AdaptiveFilter) CopyFrom(other *AdaptiveFilter) {
	n := af.filterLength
	if other.filterLength < n {
		n = other.filterLength
	}
	for i := 0; i < n; i++ {
		af.coeffs[i].CopyFrom(&other.coeffs[i])
	}
	for i := n; i < af.filterLength; i++ {
		af.coeffs[i].Clear()
	}
}

// Reset clears all filter coefficients to zero.
func (af *AdaptiveFilter) Reset() {
	for i := range af.coeffs {
		af.coeffs[i].Clear()
	}
}
