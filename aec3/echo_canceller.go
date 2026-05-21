package aec3

import (
	"sonora/fft"
)

type EchoCanceller3 struct {
	config        EchoCanceller3Config
	sampleRate    uint32
	numChannels   int
	numBands      int

	fftProcessor  *fft.OouraFFT
	renderBuffer  *RenderBuffer
	subtractor    *Subtractor
	suppressor    *SuppressionFilter
	delayEst      *DelayEstimator

	captureBlock  *Block
	renderBlock   *Block
	outputBlock   *Block

	captureFft       FftData
	renderFft        FftData
	subtractorOutput SubtractorOutput

	fftBuf [FFTSize]float32
	delay  int

	erle float32
}

func NewEchoCanceller3(config EchoCanceller3Config, sampleRate uint32, numChannels int) *EchoCanceller3 {
	numBands := numBandsForRate(sampleRate)
	ec := &EchoCanceller3{
		config:        config,
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		numBands:      numBands,
		fftProcessor:  fft.NewOouraFFT(),
		renderBuffer:  NewRenderBuffer(config.Filter.Refined.LengthBlocks + config.Delay.DelayHeadroomBlocks + 5),
		subtractor:    NewSubtractor(config.Filter),
		suppressor:    NewSuppressionFilter(config.Suppressor),
		delayEst:      NewDelayEstimator(config.Delay),
		captureBlock:  NewBlock(numBands, 1),
		renderBlock:   NewBlock(numBands, 1),
		outputBlock:   NewBlock(numBands, 1),
		delay:         config.Delay.DefaultDelayBlocks,
		erle:          1.0,
	}
	return ec
}

func (ec *EchoCanceller3) ProcessRender(renderFrame []float32) {
	if len(renderFrame) < BlockSize {
		return
	}

	copy(ec.fftBuf[:BlockSize], renderFrame[:BlockSize])
	clear(ec.fftBuf[BlockSize:])
	ec.fftProcessor.ForwardSplit(ec.fftBuf[:], ec.renderFft.Re[:], ec.renderFft.Im[:])
	ec.renderBuffer.Insert(&ec.renderFft)

	copy(ec.renderBlock.View(0, 0), renderFrame[:BlockSize])
}

func (ec *EchoCanceller3) ProcessCapture(captureFrame []float32) {
	if len(captureFrame) < BlockSize {
		return
	}

	ec.delay = ec.delayEst.Update(ec.renderBlock.View(0, 0), captureFrame[:BlockSize])

	copy(ec.fftBuf[:BlockSize], captureFrame[:BlockSize])
	clear(ec.fftBuf[BlockSize:])
	ec.fftProcessor.ForwardSplit(ec.fftBuf[:], ec.captureFft.Re[:], ec.captureFft.Im[:])

	renderPower := renderSpectrumPower(ec.renderBuffer, ec.config.Filter.Refined.LengthBlocks)
	ec.subtractor.Process(ec.renderBuffer, &ec.captureFft, renderPower, &ec.subtractorOutput)

	ec.updateErle()
	
	ec.suppressor.Process(&ec.captureFft, &ec.subtractorOutput.LinearOutput, ec.erle)

	ec.fftProcessor.InverseSplit(ec.subtractorOutput.LinearOutput.Re[:], ec.subtractorOutput.LinearOutput.Im[:], ec.fftBuf[:])
	copy(captureFrame[:BlockSize], ec.fftBuf[:BlockSize])
}

func (ec *EchoCanceller3) updateErle() {
	var capturePower, errorPower float32
	for k := 0; k < FFTSizeBy2Plus1; k++ {
		capturePower += ec.captureFft.Re[k]*ec.captureFft.Re[k] + ec.captureFft.Im[k]*ec.captureFft.Im[k]
		errorPower += ec.subtractorOutput.LinearOutput.Re[k]*ec.subtractorOutput.LinearOutput.Re[k] + ec.subtractorOutput.LinearOutput.Im[k]*ec.subtractorOutput.LinearOutput.Im[k]
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

func (ec *EchoCanceller3) ERLE() float32 {
	return ec.erle
}

func (ec *EchoCanceller3) Delay() int {
	return ec.delay
}

func (ec *EchoCanceller3) Reset() {
	ec.subtractor.Reset()
	ec.renderBuffer.Reset()
	ec.delayEst.Reset()
	ec.captureBlock.Clear()
	ec.renderBlock.Clear()
	ec.outputBlock.Clear()
	ec.captureFft.Clear()
	ec.renderFft.Clear()
	for k := 0; k < FFTSizeBy2Plus1; k++ {
		ec.subtractorOutput.RefinedError.Re[k] = 0
		ec.subtractorOutput.RefinedError.Im[k] = 0
		ec.subtractorOutput.CoarseError.Re[k] = 0
		ec.subtractorOutput.CoarseError.Im[k] = 0
		ec.subtractorOutput.LinearOutput.Re[k] = 0
		ec.subtractorOutput.LinearOutput.Im[k] = 0
	}
	ec.erle = 1.0
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
