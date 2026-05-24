package aec3

import (
	"math"
	"sonora/fft"
)

const (
	transitionSize = 30

	filterConvergenceBlocks = 250
)

// EchoCanceller3 is the main AEC3 engine.
// it processes 64-sample FloatS16 blocks, maintaining a dual adaptive filter
// (refined + coarse), a delay estimator, and a frequency-domain suppressor.
// call ProcessRender before ProcessCapture on every time step.
type EchoCanceller3 struct {
	config     EchoCanceller3Config
	sampleRate uint32
	numBands   int

	fftProcessor fft.FFT
	renderBuffer *RenderBuffer
	subtractor   *Subtractor
	suppressor   *SuppressionFilter
	cng          *ComfortNoiseGenerator
	delayEst     *DelayEstimator

	renderBlock *Block

	captureFft       FftData
	renderFft        FftData
	subtractorOutput SubtractorOutput

	fftBuf [FFTSize]float32
	delay  int
	erle   float32

	eOld                            [FFTLengthBy2]float32
	yOld                            [FFTLengthBy2]float32
	lastUsedRefined                 bool
	blockCounter                    int

	scratchYFft      FftData
	scratchEFft      FftData
	scratchCNG       FftData
	scratchNearend   [FFTSizeBy2Plus1]float32
	scratchGain      [FFTSizeBy2Plus1]float32
	scratchE2        [FFTSizeBy2Plus1]float32
	scratchY2        [FFTSizeBy2Plus1]float32
}

// NewEchoCanceller3 creates a new AEC3 engine.
// sampleRate determines the number of frequency bands (1 for <=16 kHz, 2 for <=32 kHz, 3 for 48 kHz).
// numChannels must be 1 (mono only); fftFactory is optional and selects the FFT backend.
func NewEchoCanceller3(config EchoCanceller3Config, sampleRate uint32, numChannels int, fftFactory ...fft.Factory) *EchoCanceller3 {
	factory := fft.DefaultFactory
	if len(fftFactory) > 0 && fftFactory[0] != nil {
		factory = fftFactory[0]
	}
	numBands := numBandsForRate(sampleRate)
	ec := &EchoCanceller3{
		config:       config,
		sampleRate:   sampleRate,
		numBands:     numBands,
		fftProcessor: factory(FFTSize),
		renderBuffer: NewRenderBuffer(config.Filter.Refined.LengthBlocks + config.Delay.DelayHeadroomBlocks + 5),
		subtractor:   NewSubtractor(config.Filter, factory),
		suppressor:   NewSuppressionFilter(factory),
		cng:          NewComfortNoiseGenerator(),
		delayEst:     NewDelayEstimator(config.Delay),
		renderBlock:  NewBlock(numBands, 1),
		delay:        config.Delay.DefaultDelayBlocks,
		erle:         1.0,
		lastUsedRefined: true,
	}
	return ec
}

// ProcessRender feeds one far-end (speaker/render) block into the echo canceller.
// renderFrame must contain at least BlockSize (64) FloatS16 samples.
// must be called once before each corresponding ProcessCapture call.
func (ec *EchoCanceller3) ProcessRender(renderFrame []float32) {
	if len(renderFrame) < BlockSize {
		return
	}

	clear(ec.fftBuf[:FFTLengthBy2])
	copy(ec.fftBuf[FFTLengthBy2:], renderFrame[:BlockSize])
	fft.ForwardSplit(ec.fftProcessor, ec.fftBuf[:], ec.renderFft.Re[:], ec.renderFft.Im[:])
	ec.renderBuffer.Insert(&ec.renderFft)

	copy(ec.renderBlock.View(0, 0), renderFrame[:BlockSize])
}

// ProcessCapture removes echo from a near-end (microphone) block.
// captureFrame is modified in-place and must contain at least BlockSize (64) FloatS16 samples.
// ProcessRender must have been called for the corresponding time step before this call.
func (ec *EchoCanceller3) ProcessCapture(captureFrame []float32) {
	if len(captureFrame) < BlockSize {
		return
	}

	ec.blockCounter++

	ec.delay = ec.delayEst.Update(ec.renderBlock.View(0, 0), captureFrame[:BlockSize])

	clear(ec.fftBuf[:FFTLengthBy2])
	copy(ec.fftBuf[FFTLengthBy2:], captureFrame[:BlockSize])
	fft.ForwardSplit(ec.fftProcessor, ec.fftBuf[:], ec.captureFft.Re[:], ec.captureFft.Im[:])

	renderPower := renderSpectrumPower(ec.renderBuffer, ec.config.Filter.Refined.LengthBlocks)
	ec.subtractor.Process(ec.renderBuffer, &ec.captureFft, renderPower, &ec.subtractorOutput)

	useRefined := useRefinedFilterOutput(&ec.subtractorOutput)

	var e [BlockSize]float32
	formLinearFilterOutput(
		ec.lastUsedRefined,
		useRefined,
		&ec.subtractorOutput,
		&e,
	)
	ec.lastUsedRefined = useRefined

	yFft := &ec.scratchYFft
	eFft := &ec.scratchEFft
	windowedPaddedFft(ec.fftProcessor, captureFrame[:BlockSize], &ec.yOld, yFft)
	windowedPaddedFft(ec.fftProcessor, e[:], &ec.eOld, eFft)

	ec.updateErle(yFft, eFft)

	useLinear := ec.blockCounter > filterConvergenceBlocks && ec.erle > 1.5
	inputFft := yFft
	if useLinear {
		inputFft = eFft
	}

	powerSpectrum(&inputFft.Re, &inputFft.Im, &ec.scratchNearend)

	ec.cng.Compute(false, ec.scratchNearend, &ec.scratchCNG)

	ec.computeSuppressionGain(eFft, yFft, ec.scratchGain[:])

	ec.suppressor.ApplyGain(&ec.scratchCNG, ec.scratchGain, inputFft, captureFrame[:BlockSize])
}

func useRefinedFilterOutput(o *SubtractorOutput) bool {
	if o.E2CoarseSum < 0.9*o.E2RefinedSum &&
		o.Y2 > 30*30*BlockSize &&
		(o.S2Refined > 60*60*BlockSize || o.S2Coarse > 60*60*BlockSize) {
		return false
	}
	if o.E2CoarseSum < o.E2RefinedSum && o.Y2 < o.E2RefinedSum {
		return false
	}
	return true
}

func formLinearFilterOutput(lastRefined bool, useRefined bool, o *SubtractorOutput, out *[BlockSize]float32) {
	var from, to *[BlockSize]float32
	if lastRefined {
		from = &o.ERefined
	} else {
		from = &o.ECoarse
	}
	if useRefined {
		to = &o.ERefined
	} else {
		to = &o.ECoarse
	}

	if from == to {
		*out = *to
		return
	}

	const oneByTransitionPlus1 = 1.0 / float32(transitionSize+1)
	for k := 0; k < transitionSize; k++ {
		a := float32(k+1) * oneByTransitionPlus1
		out[k] = a*to[k] + (1.0-a)*from[k]
	}
	for k := transitionSize; k < BlockSize; k++ {
		out[k] = to[k]
	}
}

func windowedPaddedFft(fftProc fft.FFT, v []float32, vOld *[FFTLengthBy2]float32, out *FftData) {
	paddedFftSqrtHanning(fftProc, v, vOld[:], out)
	copy(vOld[:], v[:FFTLengthBy2])
}

func (ec *EchoCanceller3) computeSuppressionGain(eFft, yFft *FftData, gain []float32) {
	e2 := &ec.scratchE2
	y2 := &ec.scratchY2
	powerSpectrum(&eFft.Re, &eFft.Im, e2)
	powerSpectrum(&yFft.Re, &yFft.Im, y2)

	for k := 0; k < FFTSizeBy2Plus1; k++ {
		if y2[k] > 1e-10 {
			echoPower := y2[k] - e2[k]
			if echoPower < 0 {
				echoPower = 0
			}
			residual := echoPower / ec.erle
			if residual > 1e-10 {
				snr := e2[k] / residual
				gain[k] = snr / (snr + 1.0)
			} else {
				gain[k] = 1.0
			}
		} else {
			gain[k] = 1.0
		}

		var floor float32
		if k <= 32 {
			floor = ec.config.Suppressor.NormalTuning.MaskLf.EnrTransparent
		} else {
			floor = ec.config.Suppressor.NormalTuning.MaskHf.EnrTransparent
		}
		if gain[k] < floor {
			gain[k] = floor
		}
	}
}

func (ec *EchoCanceller3) updateErle(yFft, eFft *FftData) {
	var capturePower, errorPower float32
	for k := 0; k < FFTSizeBy2Plus1; k++ {
		capturePower += yFft.Re[k]*yFft.Re[k] + yFft.Im[k]*yFft.Im[k]
		errorPower += eFft.Re[k]*eFft.Re[k] + eFft.Im[k]*eFft.Im[k]
	}

	if errorPower > 1e-10 {
		instantErle := capturePower / errorPower
		if instantErle < ec.config.Erle.Min {
			instantErle = ec.config.Erle.Min
		}
		if instantErle > ec.config.Erle.MaxLf {
			instantErle = ec.config.Erle.MaxLf
		}
		ec.erle = 0.9*ec.erle + 0.1*instantErle
	}
}

// ERLE returns the current echo return loss enhancement estimate in linear scale.
// values above 1.0 indicate that echo is being reduced; higher means more cancellation.
func (ec *EchoCanceller3) ERLE() float32 {
	return ec.erle
}

// Delay returns the estimated echo-path delay in blocks.
// multiply by BlockSize (64) to convert to samples.
func (ec *EchoCanceller3) Delay() int {
	return ec.delay
}

// Reset clears all internal state of the echo canceller.
// after a reset the engine behaves as if newly created with the same config.
func (ec *EchoCanceller3) Reset() {
	ec.subtractor.Reset()
	ec.suppressor.Reset()
	ec.renderBuffer.Reset()
	ec.delayEst.Reset()
	ec.renderBlock.Clear()
	ec.captureFft.Clear()
	ec.renderFft.Clear()
	ec.subtractorOutput = SubtractorOutput{}
	clear(ec.eOld[:])
	clear(ec.yOld[:])
	ec.erle = 1.0
	ec.lastUsedRefined = true
	ec.blockCounter = 0
}

func renderSpectrumPower(buf *RenderBuffer, filterLength int) float32 {
	var power float32
	for i := 0; i < filterLength; i++ {
		block := buf.Block(i)
		for k := 0; k < FFTSizeBy2Plus1; k++ {
			power += block.Re[k]*block.Re[k] + block.Im[k]*block.Im[k]
		}
	}
	return power
}

func computeStepSize(renderPower float32, initialScale float32) float32 {
	if renderPower < 1e-6 {
		return initialScale
	}
	step := initialScale / renderPower
	if step > 0.5 {
		step = 0.5
	}
	return step
}

func powerSpectrum(re, im *[FFTSizeBy2Plus1]float32, out *[FFTSizeBy2Plus1]float32) {
	for k := 0; k < FFTSizeBy2Plus1; k++ {
		out[k] = re[k]*re[k] + im[k]*im[k]
	}
}

func elementwiseSqrt(x []float32) {
	for i := range x {
		x[i] = float32(math.Sqrt(float64(x[i])))
	}
}
