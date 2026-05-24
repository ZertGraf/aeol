package aec3

import (
	"math"
	"sonora/fft"
)

const (
	misadjY2Threshold     = 200 * 200 * BlockSize
	misadjE2Threshold     = 7500 * 7500 * BlockSize
	misadjInvThreshold    = 10.0
	misadjOverhangBlocks  = 4
	misadjSmoothingFactor = 0.1
	misadjScaleNumerator  = 2.0
	poorCoarseThreshold   = 5
	outputClampMax        = 32767.0
	outputClampMin        = -32768.0
)

// SubtractorOutput holds the per-block output of the dual adaptive filter step.
// ERefined/ECoarse are time-domain error signals; ERefinedFft is the spectrum of ERefined.
// E2Refined/E2Coarse hold per-bin power of the respective error signals.
type SubtractorOutput struct {
	SRefined    [BlockSize]float32
	SCoarse     [BlockSize]float32
	ERefined    [BlockSize]float32
	ECoarse     [BlockSize]float32
	ERefinedFft FftData
	E2Refined   [FFTSizeBy2Plus1]float32
	E2Coarse    [FFTSizeBy2Plus1]float32

	S2Refined      float32
	S2Coarse       float32
	E2RefinedSum   float32
	E2CoarseSum    float32
	Y2             float32
	SRefinedMaxAbs float32
	SCoarseMaxAbs  float32
}

// ComputeMetrics computes energy and peak statistics from the capture signal y and
// the current ERefined/ECoarse/SRefined/SCoarse arrays stored in the receiver.
// y must be at least BlockSize (64) FloatS16 samples.
func (o *SubtractorOutput) ComputeMetrics(y []float32) {
	var y2, e2r, e2c, s2r, s2c float32
	var srMax, scMax float32
	for i := 0; i < BlockSize; i++ {
		y2 += y[i] * y[i]
		e2r += o.ERefined[i] * o.ERefined[i]
		e2c += o.ECoarse[i] * o.ECoarse[i]
		s2r += o.SRefined[i] * o.SRefined[i]
		s2c += o.SCoarse[i] * o.SCoarse[i]
		a := o.SRefined[i]
		if a < 0 {
			a = -a
		}
		if a > srMax {
			srMax = a
		}
		a = o.SCoarse[i]
		if a < 0 {
			a = -a
		}
		if a > scMax {
			scMax = a
		}
	}
	o.Y2 = y2
	o.E2RefinedSum = e2r
	o.E2CoarseSum = e2c
	o.S2Refined = s2r
	o.S2Coarse = s2c
	o.SRefinedMaxAbs = srMax
	o.SCoarseMaxAbs = scMax
}

type filterMisadjustmentEstimator struct {
	nBlocks          int
	nBlocksAccum     int
	e2Accum          float32
	y2Accum          float32
	invMisadjustment float32
	overhang         int
}

func newFilterMisadjustmentEstimator() *filterMisadjustmentEstimator {
	return &filterMisadjustmentEstimator{nBlocks: 4}
}

func (m *filterMisadjustmentEstimator) update(o *SubtractorOutput) {
	m.e2Accum += o.E2RefinedSum
	m.y2Accum += o.Y2
	m.nBlocksAccum++
	if m.nBlocksAccum == m.nBlocks {
		nb := float32(m.nBlocks)
		if m.y2Accum > nb*misadjY2Threshold {
			update := m.e2Accum / m.y2Accum
			if m.e2Accum > nb*misadjE2Threshold {
				m.overhang = misadjOverhangBlocks
			} else if m.overhang > 0 {
				m.overhang--
			}
			if update < m.invMisadjustment || m.overhang > 0 {
				m.invMisadjustment += misadjSmoothingFactor * (update - m.invMisadjustment)
			}
		}
		m.e2Accum = 0
		m.y2Accum = 0
		m.nBlocksAccum = 0
	}
}

func (m *filterMisadjustmentEstimator) adjustmentNeeded() bool {
	return m.invMisadjustment > misadjInvThreshold
}

func (m *filterMisadjustmentEstimator) misadjustment() float32 {
	return misadjScaleNumerator / float32(math.Sqrt(float64(m.invMisadjustment)))
}

func (m *filterMisadjustmentEstimator) reset() {
	m.e2Accum = 0
	m.y2Accum = 0
	m.nBlocksAccum = 0
	m.invMisadjustment = 0
	m.overhang = 0
}

// Subtractor runs both the refined and coarse NLMS adaptive filters each block.
// the refined filter tracks the echo path precisely; the coarse filter provides
// a stable fallback when the refined filter diverges or misadjusts.
type Subtractor struct {
	refinedFilter *AdaptiveFilter
	coarseFilter  *AdaptiveFilter
	config        FilterConfig
	fftProcessor  fft.FFT

	misadjEstimator       *filterMisadjustmentEstimator
	poorCoarseFilterCount int
	coarseResetHangover   int

	scratchSRefined   FftData
	scratchSCoarse    FftData
	scratchECoarseFft FftData
}

// NewSubtractor creates a Subtractor using the given filter config.
// fftFactory is optional; if omitted the default Ooura FFT backend is used.
func NewSubtractor(config FilterConfig, fftFactory ...fft.Factory) *Subtractor {
	factory := fft.DefaultFactory
	if len(fftFactory) > 0 && fftFactory[0] != nil {
		factory = fftFactory[0]
	}
	return &Subtractor{
		refinedFilter:   NewAdaptiveFilter(config.Refined.LengthBlocks),
		coarseFilter:    NewAdaptiveFilter(config.Coarse.LengthBlocks),
		config:          config,
		fftProcessor:    factory(FFTSize),
		misadjEstimator: newFilterMisadjustmentEstimator(),
	}
}

func predictionError(fftProc fft.FFT, s *FftData, y []float32, e *[BlockSize]float32, sOut *[BlockSize]float32, tmp *[FFTSize]float32) {
	fft.InverseSplit(fftProc, s.Re[:], s.Im[:], tmp[:])
	for k := 0; k < BlockSize; k++ {
		e[k] = y[k] - tmp[k+FFTLengthBy2]
	}
	if sOut != nil {
		for k := 0; k < BlockSize; k++ {
			sOut[k] = tmp[k+FFTLengthBy2]
		}
	}
}

func scaleFilterOutput(y []float32, factor float32, e *[BlockSize]float32, s *[BlockSize]float32) {
	for k := 0; k < BlockSize; k++ {
		s[k] *= factor
		e[k] = y[k] - s[k]
	}
}

func (sub *Subtractor) Process(renderBuffer *RenderBuffer, captureFft *FftData, renderPower float32, output *SubtractorOutput) {
	var y [BlockSize]float32
	var iFftBuf [FFTSize]float32
	fft.InverseSplit(sub.fftProcessor, captureFft.Re[:], captureFft.Im[:], iFftBuf[:])
	for k := 0; k < BlockSize; k++ {
		y[k] = iFftBuf[k+FFTLengthBy2]
	}

	sub.refinedFilter.Filter(renderBuffer, &sub.scratchSRefined)
	sub.coarseFilter.Filter(renderBuffer, &sub.scratchSCoarse)

	predictionError(sub.fftProcessor, &sub.scratchSRefined, y[:], &output.ERefined, &output.SRefined, &iFftBuf)
	predictionError(sub.fftProcessor, &sub.scratchSCoarse, y[:], &output.ECoarse, &output.SCoarse, &iFftBuf)

	output.ComputeMetrics(y[:])

	sub.misadjEstimator.update(output)
	adjusted := false
	if sub.misadjEstimator.adjustmentNeeded() {
		scale := sub.misadjEstimator.misadjustment()
		sub.refinedFilter.ScaleFilter(scale)
		scaleFilterOutput(y[:], scale, &output.ERefined, &output.SRefined)
		sub.misadjEstimator.reset()
		adjusted = true
	}

	zeroPaddedFft(sub.fftProcessor, output.ERefined[:], &output.ERefinedFft)
	zeroPaddedFft(sub.fftProcessor, output.ECoarse[:], &sub.scratchECoarseFft)

	powerSpectrum(&output.ERefinedFft.Re, &output.ERefinedFft.Im, &output.E2Refined)
	powerSpectrum(&sub.scratchECoarseFft.Re, &sub.scratchECoarseFft.Im, &output.E2Coarse)

	if !adjusted {
		refinedStep := computeStepSize(renderPower, sub.config.Refined.InitialScale)
		sub.refinedFilter.Adapt(renderBuffer, &output.ERefinedFft, refinedStep)
	}

	if output.E2RefinedSum < output.E2CoarseSum {
		sub.poorCoarseFilterCount++
	} else {
		sub.poorCoarseFilterCount = 0
	}

	if sub.coarseResetHangover > 0 {
		sub.coarseResetHangover--
	}

	if sub.poorCoarseFilterCount < poorCoarseThreshold || sub.coarseResetHangover > 0 {
		coarseStep := computeStepSize(renderPower, sub.config.Coarse.InitialScale)
		sub.coarseFilter.Adapt(renderBuffer, &sub.scratchECoarseFft, coarseStep)
	} else {
		sub.poorCoarseFilterCount = 0
		sub.coarseFilter.CopyFrom(sub.refinedFilter)
		coarseStep := computeStepSize(renderPower, sub.config.Coarse.InitialScale)
		sub.coarseFilter.Adapt(renderBuffer, &output.ERefinedFft, coarseStep)
		sub.coarseResetHangover = sub.config.CoarseResetHangoverBlocks
	}

	for k := 0; k < BlockSize; k++ {
		if output.ERefined[k] > outputClampMax {
			output.ERefined[k] = outputClampMax
		} else if output.ERefined[k] < outputClampMin {
			output.ERefined[k] = outputClampMin
		}
		if output.ECoarse[k] > outputClampMax {
			output.ECoarse[k] = outputClampMax
		} else if output.ECoarse[k] < outputClampMin {
			output.ECoarse[k] = outputClampMin
		}
	}
}

// Reset clears both adaptive filters and all estimation state.
func (sub *Subtractor) Reset() {
	sub.refinedFilter.Reset()
	sub.coarseFilter.Reset()
	sub.misadjEstimator.reset()
	sub.poorCoarseFilterCount = 0
	sub.coarseResetHangover = 0
}
