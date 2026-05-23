package aec3

type AdaptiveFilter struct {
	filterLength int
	coeffs       []FftData
}

func NewAdaptiveFilter(lengthBlocks int) *AdaptiveFilter {
	return &AdaptiveFilter{
		filterLength: lengthBlocks,
		coeffs:       make([]FftData, lengthBlocks),
	}
}

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

func (af *AdaptiveFilter) ScaleFilter(scale float32) {
	for i := range af.coeffs {
		c := &af.coeffs[i]
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			c.Re[k] *= scale
			c.Im[k] *= scale
		}
	}
}

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

func (af *AdaptiveFilter) Reset() {
	for i := range af.coeffs {
		af.coeffs[i].Clear()
	}
}
