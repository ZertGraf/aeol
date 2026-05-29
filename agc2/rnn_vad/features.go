package rnn_vad

import (
	"math"

	"github.com/ZertGraf/aeol/dsp"
	"github.com/ZertGraf/aeol/fft"
)

const kOpusBands24kHz = 20

var kOpusScaleNumBins24kHz20ms = [kOpusBands24kHz - 1]int{
	4, 4, 4, 4, 4, 4, 4, 4, 8, 8, 8, 8, 16, 16, 16, 24, 24, 32, 48,
}

var kOpusBandWeights24kHz20ms = [frameSize20ms24kHz / 2]float32{
	0, 0.25, 0.5, 0.75, // Band 0
	0, 0.25, 0.5, 0.75, // Band 1
	0, 0.25, 0.5, 0.75, // Band 2
	0, 0.25, 0.5, 0.75, // Band 3
	0, 0.25, 0.5, 0.75, // Band 4
	0, 0.25, 0.5, 0.75, // Band 5
	0, 0.25, 0.5, 0.75, // Band 6
	0, 0.25, 0.5, 0.75, // Band 7
	0, 0.125, 0.25, 0.375, 0.5, 0.625, 0.75, 0.875, // Band 8
	0, 0.125, 0.25, 0.375, 0.5, 0.625, 0.75, 0.875, // Band 9
	0, 0.125, 0.25, 0.375, 0.5, 0.625, 0.75, 0.875, // Band 10
	0, 0.125, 0.25, 0.375, 0.5, 0.625, 0.75, 0.875, // Band 11
	0, 0.0625, 0.125, 0.1875, 0.25, 0.3125, 0.375, 0.4375, 0.5, 0.5625, 0.625, 0.6875, 0.75, 0.8125, 0.875, 0.9375, // Band 12
	0, 0.0625, 0.125, 0.1875, 0.25, 0.3125, 0.375, 0.4375, 0.5, 0.5625, 0.625, 0.6875, 0.75, 0.8125, 0.875, 0.9375, // Band 13
	0, 0.0625, 0.125, 0.1875, 0.25, 0.3125, 0.375, 0.4375, 0.5, 0.5625, 0.625, 0.6875, 0.75, 0.8125, 0.875, 0.9375, // Band 14
	0, 0.0416667, 0.0833333, 0.125, 0.166667, 0.208333, 0.25, 0.291667, 0.333333, 0.375, 0.416667, 0.458333, 0.5, 0.541667, 0.583333, 0.625, 0.666667, 0.708333, 0.75, 0.791667, 0.833333, 0.875, 0.916667, 0.958333, // Band 15
	0, 0.0416667, 0.0833333, 0.125, 0.166667, 0.208333, 0.25, 0.291667, 0.333333, 0.375, 0.416667, 0.458333, 0.5, 0.541667, 0.583333, 0.625, 0.666667, 0.708333, 0.75, 0.791667, 0.833333, 0.875, 0.916667, 0.958333, // Band 16
	0, 0.03125, 0.0625, 0.09375, 0.125, 0.15625, 0.1875, 0.21875, 0.25, 0.28125, 0.3125, 0.34375, 0.375, 0.40625, 0.4375, 0.46875, 0.5, 0.53125, 0.5625, 0.59375, 0.625, 0.65625, 0.6875, 0.71875, 0.75, 0.78125, 0.8125, 0.84375, 0.875, 0.90625, 0.9375, 0.96875, // Band 17
	0, 0.0208333, 0.0416667, 0.0625, 0.0833333, 0.104167, 0.125, 0.145833, 0.166667, 0.1875, 0.208333, 0.229167, 0.25, 0.270833, 0.291667, 0.3125, 0.333333, 0.354167, 0.375, 0.395833, 0.416667, 0.4375, 0.458333, 0.479167, 0.5, 0.520833, 0.541667, 0.5625, 0.583333, 0.604167, 0.625, 0.645833, 0.666667, 0.6875, 0.708333, 0.729167, 0.75, 0.770833, 0.791667, 0.8125, 0.833333, 0.854167, 0.875, 0.895833, 0.916667, 0.9375, 0.958333, 0.979167, // Band 18
}

type SpectralCorrelator struct{}

func (s *SpectralCorrelator) computeCrossCorrelation(x, y []float32, crossCorr []float32) {
	k := 0
	crossCorr[0] = 0
	for i := 0; i < kOpusBands24kHz-1; i++ {
		crossCorr[i+1] = 0
		for j := 0; j < kOpusScaleNumBins24kHz20ms[i]; j++ {
			v := x[2*k]*y[2*k] + x[2*k+1]*y[2*k+1]
			tmp := kOpusBandWeights24kHz20ms[k] * v
			crossCorr[i] += v - tmp
			crossCorr[i+1] += tmp
			k++
		}
	}
	crossCorr[0] *= 2.0
}

func (s *SpectralCorrelator) computeAutoCorrelation(x []float32, autoCorr []float32) {
	s.computeCrossCorrelation(x, x, autoCorr)
}

type sequenceBuffer struct {
	buf [bufSize24kHz]float32
}

func (b *sequenceBuffer) reset() {
	clear(b.buf[:])
}

func (b *sequenceBuffer) getBufferView() []float32 {
	return b.buf[:]
}

func (b *sequenceBuffer) getMostRecentValuesView() []float32 {
	return b.buf[bufSize24kHz-frameSize20ms24kHz:]
}

func (b *sequenceBuffer) push(newValues []float32) {
	n := len(newValues)
	copy(b.buf[:], b.buf[n:])
	copy(b.buf[bufSize24kHz-n:], newValues)
}

type ringBuffer struct {
	tail int
	buf  [cepstralHistorySize * numBands]float32
}

func (r *ringBuffer) reset() {
	r.tail = 0
	clear(r.buf[:])
}

func (r *ringBuffer) push(newValues []float32) {
	copy(r.buf[numBands*r.tail:], newValues)
	r.tail++
	if r.tail == cepstralHistorySize {
		r.tail = 0
	}
}

func (r *ringBuffer) getArrayView(delay int) []float32 {
	offset := r.tail - 1 - delay
	if offset < 0 {
		offset += cepstralHistorySize
	}
	return r.buf[numBands*offset : numBands*(offset+1)]
}

type symmetricMatrixBuffer struct {
	buf [(cepstralHistorySize - 1) * (cepstralHistorySize - 1)]float32
}

func (s *symmetricMatrixBuffer) reset() {
	clear(s.buf[:])
}

func (s *symmetricMatrixBuffer) push(values []float32) {
	S := cepstralHistorySize
	copy(s.buf[:], s.buf[S:])
	for i := 0; i < len(values); i++ {
		index := (S-1-i)*(S-1) - 1
		s.buf[index] = values[i]
	}
}

func (s *symmetricMatrixBuffer) getValue(delay1, delay2 int) float32 {
	S := cepstralHistorySize
	row := S - 1 - delay1
	col := S - 1 - delay2
	if row > col {
		row, col = col, row
	}
	index := row*(S-1) + (col - 1)
	return s.buf[index]
}

func computeScaledHalfVorbisWindow(scaling float32) [frameSize20ms24kHz / 2]float32 {
	var halfWindow [frameSize20ms24kHz / 2]float32
	halfSize := float64(frameSize20ms24kHz / 2)
	for i := 0; i < int(halfSize); i++ {
		x := math.Sin(0.5 * math.Pi * (float64(i) + 0.5) / halfSize)
		halfWindow[i] = scaling * float32(math.Sin(0.5*math.Pi*x*x))
	}
	return halfWindow
}

func computeSmoothedLogMagnitudeSpectrum(bandsEnergy []float32, logBandsEnergy []float32) {
	const kOneByHundred = 1e-2
	const kLogOneByHundred = -2.0

	logMax := float32(kLogOneByHundred)
	follow := float32(kLogOneByHundred)

	smooth := func(x float32) float32 {
		max1 := logMax - 7.0
		if follow-1.5 > max1 {
			max1 = follow - 1.5
		}
		if x > max1 {
			max1 = x
		}
		x = max1

		if x > logMax {
			logMax = x
		}
		if x > follow-1.5 {
			follow = x
		} else {
			follow = follow - 1.5
		}
		return x
	}

	for i := 0; i < len(bandsEnergy); i++ {
		logBandsEnergy[i] = smooth(float32(math.Log10(kOneByHundred + float64(bandsEnergy[i]))))
	}
	for i := len(bandsEnergy); i < numBands; i++ {
		logBandsEnergy[i] = smooth(kLogOneByHundred)
	}
}

func computeDctTable() [numBands * numBands]float32 {
	var dctTable [numBands * numBands]float32
	k := float32(math.Sqrt(0.5))
	for i := 0; i < numBands; i++ {
		for j := 0; j < numBands; j++ {
			dctTable[i*numBands+j] = float32(math.Cos((float64(i) + 0.5) * float64(j) * math.Pi / float64(numBands)))
		}
		dctTable[i*numBands] *= k
	}
	return dctTable
}

func computeDct(in []float32, dctTable []float32, out []float32) {
	const kDctScalingFactor = float32(0.301511345)
	for i := 0; i < len(out); i++ {
		var sum float32
		for j := 0; j < len(in); j++ {
			sum += in[j] * dctTable[j*numBands+i]
		}
		out[i] = sum * kDctScalingFactor
	}
}

func updateCepstralDifferenceStats(newCepstralCoeffs []float32, ringBuf *ringBuffer, symMatrixBuf *symmetricMatrixBuffer) {
	var distances [cepstralHistorySize - 1]float32
	for i := 0; i < cepstralHistorySize-1; i++ {
		delay := i + 1
		oldCepstralCoeffs := ringBuf.getArrayView(delay)
		distances[i] = 0
		for k := 0; k < numBands; k++ {
			c := newCepstralCoeffs[k] - oldCepstralCoeffs[k]
			distances[i] += c * c
		}
	}
	symMatrixBuf.push(distances[:])
}

type SpectralFeaturesExtractor struct {
	halfWindow                [frameSize20ms24kHz / 2]float32
	fft480                    fft.FFT
	referenceFrameFft         [frameSize20ms24kHz]float32
	laggedFrameFft            [frameSize20ms24kHz]float32
	spectralCorrelator        *SpectralCorrelator
	referenceFrameBandsEnergy [kOpusBands24kHz]float32
	laggedFrameBandsEnergy    [kOpusBands24kHz]float32
	bandsCrossCorr            [kOpusBands24kHz]float32
	dctTable                  [numBands * numBands]float32
	cepstralCoeffsRingBuf     ringBuffer
	cepstralDiffsBuf          symmetricMatrixBuffer
}

func NewSpectralFeaturesExtractor(fftFactory ...fft.Factory) *SpectralFeaturesExtractor {
	factory := fft.DefaultFactory
	if len(fftFactory) > 0 && fftFactory[0] != nil {
		factory = fftFactory[0]
	}
	return &SpectralFeaturesExtractor{
		halfWindow:         computeScaledHalfVorbisWindow(1.0 / frameSize20ms24kHz),
		fft480:             factory(frameSize20ms24kHz),
		spectralCorrelator: &SpectralCorrelator{},
		dctTable:           computeDctTable(),
	}
}

func (s *SpectralFeaturesExtractor) Reset() {
	s.cepstralCoeffsRingBuf.reset()
	s.cepstralDiffsBuf.reset()
}

func (s *SpectralFeaturesExtractor) computeWindowedForwardFft(frame []float32, fftOutput []float32) {
	halfSize := frameSize20ms24kHz / 2
	for i, j := 0, frameSize20ms24kHz-1; i < halfSize; i, j = i+1, j-1 {
		fftOutput[i] = frame[i] * s.halfWindow[i]
		fftOutput[j] = frame[j] * s.halfWindow[i]
	}
	s.fft480.Forward(fftOutput[:frameSize20ms24kHz])
	fftOutput[1] = 0.0
}

func (s *SpectralFeaturesExtractor) CheckSilenceComputeFeatures(
	referenceFrame []float32,
	laggedFrame []float32,
	higherBandsCepstrum []float32,
	average []float32,
	firstDerivative []float32,
	secondDerivative []float32,
	bandsCrossCorr []float32,
	variability *float32,
) bool {
	s.computeWindowedForwardFft(referenceFrame, s.referenceFrameFft[:])
	s.spectralCorrelator.computeAutoCorrelation(s.referenceFrameFft[:], s.referenceFrameBandsEnergy[:])

	var totEnergy float32
	for _, e := range s.referenceFrameBandsEnergy {
		totEnergy += e
	}
	if totEnergy < 0.04 {
		return true
	}

	s.computeWindowedForwardFft(laggedFrame, s.laggedFrameFft[:])
	s.spectralCorrelator.computeAutoCorrelation(s.laggedFrameFft[:], s.laggedFrameBandsEnergy[:])

	var logBandsEnergy [numBands]float32
	computeSmoothedLogMagnitudeSpectrum(s.referenceFrameBandsEnergy[:], logBandsEnergy[:])

	var cepstrum [numBands]float32
	computeDct(logBandsEnergy[:], s.dctTable[:], cepstrum[:])

	cepstrum[0] -= 12.0
	cepstrum[1] -= 4.0

	s.cepstralCoeffsRingBuf.push(cepstrum[:])
	updateCepstralDifferenceStats(cepstrum[:], &s.cepstralCoeffsRingBuf, &s.cepstralDiffsBuf)

	copy(higherBandsCepstrum, cepstrum[numLowerBands:])

	s.computeAvgAndDerivatives(average, firstDerivative, secondDerivative)
	s.computeNormalizedCepstralCorrelation(bandsCrossCorr)

	*variability = s.computeVariability()
	return false
}

func (s *SpectralFeaturesExtractor) computeAvgAndDerivatives(average, firstDerivative, secondDerivative []float32) {
	curr := s.cepstralCoeffsRingBuf.getArrayView(0)
	prev1 := s.cepstralCoeffsRingBuf.getArrayView(1)
	prev2 := s.cepstralCoeffsRingBuf.getArrayView(2)
	for i := 0; i < len(average); i++ {
		average[i] = curr[i] + prev1[i] + prev2[i]
		firstDerivative[i] = curr[i] - prev2[i]
		secondDerivative[i] = curr[i] - 2*prev1[i] + prev2[i]
	}
}

func (s *SpectralFeaturesExtractor) computeNormalizedCepstralCorrelation(bandsCrossCorr []float32) {
	s.spectralCorrelator.computeCrossCorrelation(s.referenceFrameFft[:], s.laggedFrameFft[:], s.bandsCrossCorr[:])
	for i := 0; i < len(s.bandsCrossCorr); i++ {
		s.bandsCrossCorr[i] /= float32(math.Sqrt(0.001 + float64(s.referenceFrameBandsEnergy[i]*s.laggedFrameBandsEnergy[i])))
	}
	computeDct(s.bandsCrossCorr[:], s.dctTable[:], bandsCrossCorr)
	bandsCrossCorr[0] -= 1.3
	bandsCrossCorr[1] -= 0.9
}

func (s *SpectralFeaturesExtractor) computeVariability() float32 {
	var variability float32
	for delay1 := 0; delay1 < cepstralHistorySize; delay1++ {
		minDist := float32(math.MaxFloat32)
		for delay2 := 0; delay2 < cepstralHistorySize; delay2++ {
			if delay1 == delay2 {
				continue
			}
			dist := s.cepstralDiffsBuf.getValue(delay1, delay2)
			if dist < minDist {
				minDist = dist
			}
		}
		variability += minDist
	}
	return variability/cepstralHistorySize - 2.1
}

type FeaturesExtractor struct {
	useHighPassFilter         bool
	hpf                       *dsp.BiQuadFilter
	pitchBuf24kHz             sequenceBuffer
	lpResidual                [bufSize24kHz]float32
	pitchEstimator            *pitchEstimator
	spectralFeaturesExtractor *SpectralFeaturesExtractor
	pitchPeriod48kHz          int
}

func NewFeaturesExtractor(fftFactory ...fft.Factory) *FeaturesExtractor {
	return &FeaturesExtractor{
		useHighPassFilter: false,
		hpf: dsp.NewBiQuadFilter(dsp.BiQuadCoefficients{
			B: [3]float32{0.99446179, -1.98892358, 0.99446179},
			A: [2]float32{-1.98889291, 0.98895425},
		}),
		pitchEstimator:            newPitchEstimator(),
		spectralFeaturesExtractor: NewSpectralFeaturesExtractor(fftFactory...),
	}
}

func (f *FeaturesExtractor) Reset() {
	f.pitchBuf24kHz.reset()
	f.spectralFeaturesExtractor.Reset()
	if f.useHighPassFilter {
		f.hpf.Reset()
	}
}

func (f *FeaturesExtractor) CheckSilenceComputeFeatures(samples []float32, featureVector []float32) bool {
	if f.useHighPassFilter {
		var samplesFiltered [frameSize10ms24kHz]float32
		f.hpf.Process(samples, samplesFiltered[:])
		f.pitchBuf24kHz.push(samplesFiltered[:])
	} else {
		f.pitchBuf24kHz.push(samples)
	}

	pitchBufView := f.pitchBuf24kHz.getBufferView()
	var lpcCoeffs [numLpcCoefficients]float32
	computeAndPostProcessLpcCoefficients(pitchBufView, lpcCoeffs[:])
	computeLpResidual(lpcCoeffs, pitchBufView, f.lpResidual[:])

	f.pitchPeriod48kHz = f.pitchEstimator.estimate(f.lpResidual[:])
	featureVector[featureVectorSize-2] = 0.01 * float32(f.pitchPeriod48kHz-300)

	lag := maxPitch24kHz - f.pitchPeriod48kHz/2
	laggedFrame := pitchBufView[lag : lag+frameSize20ms24kHz]
	referenceFrame := f.pitchBuf24kHz.getMostRecentValuesView()

	higherBandsCepstrum := featureVector[numLowerBands:numBands]
	average := featureVector[:numLowerBands]
	firstDerivative := featureVector[numBands : numBands+numLowerBands]
	secondDerivative := featureVector[numBands+numLowerBands : numBands+2*numLowerBands]
	bandsCrossCorr := featureVector[numBands+2*numLowerBands : numBands+3*numLowerBands]
	variability := &featureVector[featureVectorSize-1]

	return f.spectralFeaturesExtractor.CheckSilenceComputeFeatures(
		referenceFrame, laggedFrame,
		higherBandsCepstrum, average, firstDerivative, secondDerivative,
		bandsCrossCorr, variability,
	)
}