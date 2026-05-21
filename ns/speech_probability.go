package ns

import "math"

type priorModel struct {
	lrtWeight     float32
	flatWeight    float32
	diffWeight    float32
	lrtThreshold  float32
	flatThreshold float32
	diffThreshold float32
}

func defaultPriorModel() priorModel {
	return priorModel{
		lrtWeight:     1.0,
		flatWeight:    0.0,
		diffWeight:    0.0,
		lrtThreshold:  1.2,
		flatThreshold: 0.0,
		diffThreshold: 0.0,
	}
}

type histograms struct {
	lrt  [100]int
	flat [100]int
	diff [100]int
}

type speechProbabilityEstimator struct {
	prior      priorModel
	hist       histograms
	frameCount int
}

func newSpeechProbabilityEstimator() *speechProbabilityEstimator {
	return &speechProbabilityEstimator{
		prior: defaultPriorModel(),
	}
}

func (spe *speechProbabilityEstimator) Estimate(
	signalModel *signalModel,
	noiseSpectrum [numFreqBins]float32,
	signalSpectrum [numFreqBins]float32,
	speechProb []float32,
) {
	spe.frameCount++

	for i := 0; i < numFreqBins; i++ {
		lrt := signalModel.lrt[i]
		indicator := spe.prior.lrtWeight * (lrt - spe.prior.lrtThreshold)

		if signalModel.spectralFlatness > 0 {
			indicator += spe.prior.flatWeight *
				(signalModel.spectralFlatness - spe.prior.flatThreshold)
		}
		if signalModel.spectralDiff > 0 {
			indicator += spe.prior.diffWeight *
				(signalModel.spectralDiff - spe.prior.diffThreshold)
		}

		prob := float32(1.0 / (1.0 + math.Exp(-float64(indicator))))
		speechProb[i] = prob
	}
}

type signalModelEstimator struct {
	prevMagnitude [numFreqBins]float32
	frameCount    int
}

func newSignalModelEstimator() *signalModelEstimator {
	return &signalModelEstimator{}
}

func (sme *signalModelEstimator) Update(
	signalSpectrum [numFreqBins]float32,
	noiseSpectrum [numFreqBins]float32,
	model *signalModel,
) {
	sme.frameCount++

	var sumLrt float64
	for i := 0; i < numFreqBins; i++ {
		snr := signalSpectrum[i] - noiseSpectrum[i]
		if snr < 0 {
			snr = 0
		}
		model.lrt[i] = snr
		sumLrt += float64(snr)
	}
	model.avgLogLrt = float32(sumLrt / numFreqBins)

	if sme.frameCount > 1 {
		var sumSig, sumDiff float64
		for i := 0; i < numFreqBins; i++ {
			mag := fastExp(0.5 * signalSpectrum[i])
			sumSig += float64(mag)
			d := mag - sme.prevMagnitude[i]
			if d < 0 {
				d = -d
			}
			sumDiff += float64(d)
			sme.prevMagnitude[i] = mag
		}
		if sumSig > 0 {
			model.spectralDiff = float32(sumDiff / sumSig)
		}
	} else {
		for i := 0; i < numFreqBins; i++ {
			sme.prevMagnitude[i] = fastExp(0.5 * signalSpectrum[i])
		}
	}

	var geoMean, ariMean float64
	for i := 1; i < numFreqBins; i++ {
		mag := fastExp(signalSpectrum[i])
		if mag < 1e-10 {
			mag = 1e-10
		}
		geoMean += float64(fastLog(mag))
		ariMean += float64(mag)
	}
	n := float64(numFreqBins - 1)
	geoMean = math.Exp(geoMean / n)
	ariMean /= n

	if ariMean > 0 {
		model.spectralFlatness = float32(geoMean / ariMean)
	}
}
