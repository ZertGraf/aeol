package aec3

type Subtractor struct {
	RefinedFilter *AdaptiveFilter
	CoarseFilter  *AdaptiveFilter
	config        FilterConfig
}

func NewSubtractor(config FilterConfig) *Subtractor {
	return &Subtractor{
		RefinedFilter: NewAdaptiveFilter(config.Refined.LengthBlocks),
		CoarseFilter:  NewAdaptiveFilter(config.Coarse.LengthBlocks),
		config:        config,
	}
}

// SubtractorOutput holds the result of the dual-filter subtraction
type SubtractorOutput struct {
	RefinedError FftData
	CoarseError  FftData
	LinearOutput FftData
}

func (s *Subtractor) Process(renderBuffer *RenderBuffer, captureFft *FftData, renderPower float32, output *SubtractorOutput) {
	var refinedOutput FftData
	var coarseOutput FftData

	s.RefinedFilter.Filter(renderBuffer, &refinedOutput)
	s.CoarseFilter.Filter(renderBuffer, &coarseOutput)

	var refinedErrorPower, coarseErrorPower float32

	for k := 0; k < FFTSizeBy2Plus1; k++ {
		// Calculate Refined error
		output.RefinedError.Re[k] = captureFft.Re[k] - refinedOutput.Re[k]
		output.RefinedError.Im[k] = captureFft.Im[k] - refinedOutput.Im[k]
		refinedErrorPower += output.RefinedError.Re[k]*output.RefinedError.Re[k] + output.RefinedError.Im[k]*output.RefinedError.Im[k]

		// Calculate Coarse error
		output.CoarseError.Re[k] = captureFft.Re[k] - coarseOutput.Re[k]
		output.CoarseError.Im[k] = captureFft.Im[k] - coarseOutput.Im[k]
		coarseErrorPower += output.CoarseError.Re[k]*output.CoarseError.Re[k] + output.CoarseError.Im[k]*output.CoarseError.Im[k]
	}

	// Choose the output with the lower error power
	if coarseErrorPower < refinedErrorPower {
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			output.LinearOutput.Re[k] = output.CoarseError.Re[k]
			output.LinearOutput.Im[k] = output.CoarseError.Im[k]
		}
	} else {
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			output.LinearOutput.Re[k] = output.RefinedError.Re[k]
			output.LinearOutput.Im[k] = output.RefinedError.Im[k]
		}
	}

	// Adapt filters
	refinedStep := computeStepSize(renderPower, s.config.Refined.InitialScale)
	s.RefinedFilter.Adapt(renderBuffer, &output.RefinedError, refinedStep)

	coarseStep := computeStepSize(renderPower, s.config.Coarse.InitialScale)
	s.CoarseFilter.Adapt(renderBuffer, &output.CoarseError, coarseStep)
}

func (s *Subtractor) Reset() {
	s.RefinedFilter.Reset()
	s.CoarseFilter.Reset()
}
