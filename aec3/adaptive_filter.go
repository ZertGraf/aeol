package aec3

import "sonora/simd"

type AdaptiveFilter struct {
	filterLength int
	coeffs       []*FftData
	backend      simd.Backend
}

func NewAdaptiveFilter(lengthBlocks int) *AdaptiveFilter {
	coeffs := make([]*FftData, lengthBlocks)
	for i := range coeffs {
		coeffs[i] = &FftData{}
	}
	return &AdaptiveFilter{
		filterLength: lengthBlocks,
		coeffs:       coeffs,
		backend:      simd.Default(),
	}
}

func (af *AdaptiveFilter) Filter(renderBuffer *RenderBuffer, output *FftData) {
	output.Clear()
	for i := 0; i < af.filterLength; i++ {
		renderBlock := renderBuffer.Block(i)
		af.backend.ComplexMultiplyAccumulate(
			af.coeffs[i].Re[:], af.coeffs[i].Im[:],
			renderBlock.Re[:], renderBlock.Im[:],
			output.Re[:], output.Im[:],
		)
	}
}

func (af *AdaptiveFilter) Adapt(renderBuffer *RenderBuffer, error *FftData, stepSize float32) {
	for i := 0; i < af.filterLength; i++ {
		renderBlock := renderBuffer.Block(i)
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			af.coeffs[i].Re[k] += stepSize * (error.Re[k]*renderBlock.Re[k] + error.Im[k]*renderBlock.Im[k])
			af.coeffs[i].Im[k] += stepSize * (-error.Re[k]*renderBlock.Im[k] + error.Im[k]*renderBlock.Re[k])
		}
	}
}

func (af *AdaptiveFilter) Energy() float32 {
	var energy float32
	for _, c := range af.coeffs {
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			energy += c.Re[k]*c.Re[k] + c.Im[k]*c.Im[k]
		}
	}
	return energy
}

func (af *AdaptiveFilter) Reset() {
	for _, c := range af.coeffs {
		c.Clear()
	}
}
