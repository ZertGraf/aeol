package ns

import "math"

// histBins is the number of histogram buckets for each feature.
const histBins = 100

// histAdaptFrames is the number of frames collected before the first
// threshold/weight adaptation pass (matches WebRTC NS: ~200 frames = 2s).
const histAdaptFrames = 200

// feature range constants used to map raw values into [0, histBins) buckets.
const (
	// lrt ranges in [0, lrtMax]; typical speech ≥ 1.0, noise < 0.5.
	lrtMax = 8.0
	// flatness is in [0, 1]; flat spectra (noise) approach 1.
	flatMax = 1.0
	// spectral diff is normalised; speech has larger frame-to-frame changes.
	diffMax = 1.0
)

// priorModel holds the per-feature weights and thresholds used to combine
// per-bin LRT with frame-level spectral flatness and spectral difference
// into a single speech/noise probability per bin.
type priorModel struct {
	lrtWeight     float32
	flatWeight    float32
	diffWeight    float32
	lrtThreshold  float32
	flatThreshold float32
	diffThreshold float32
}

// defaultPriorModel returns initial weights derived from WebRTC NS values.
// lrt is the primary feature; flatness and diff contribute additively.
// thresholds are tuned for the first ~200 frames before histogram adaptation.
func defaultPriorModel() priorModel {
	return priorModel{
		lrtWeight:     1.0,
		flatWeight:    0.5,
		diffWeight:    0.5,
		lrtThreshold:  1.0,
		flatThreshold: 0.5,
		diffThreshold: 0.3,
	}
}

// histograms accumulates per-feature sample counts in fixed-width buckets.
// each field is indexed [0, histBins); counts are reset after each adaptation.
type histograms struct {
	lrt  [histBins]int
	flat [histBins]int
	diff [histBins]int
}

// speechProbabilityEstimator combines a prior model with adaptive histogram
// thresholds to estimate per-bin speech probability.
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

// binLRT maps a raw lrt value into a histogram bucket index [0, histBins).
func binLRT(v float32) int {
	idx := int(v * histBins / lrtMax)
	if idx < 0 {
		return 0
	}
	if idx >= histBins {
		return histBins - 1
	}
	return idx
}

// binFlat maps a spectral flatness value into a histogram bucket index.
func binFlat(v float32) int {
	idx := int(v * histBins / flatMax)
	if idx < 0 {
		return 0
	}
	if idx >= histBins {
		return histBins - 1
	}
	return idx
}

// binDiff maps a spectral diff value into a histogram bucket index.
func binDiff(v float32) int {
	idx := int(v * histBins / diffMax)
	if idx < 0 {
		return 0
	}
	if idx >= histBins {
		return histBins - 1
	}
	return idx
}

// updateHistograms records feature values into the running histograms.
// lrtVal is the mean per-bin LRT for this frame (model.avgLrt).
// flatVal is spectral flatness [0,1], diffVal is spectral diff [0,1].
func (spe *speechProbabilityEstimator) updateHistograms(lrtVal, flatVal, diffVal float32) {
	spe.hist.lrt[binLRT(lrtVal)]++
	spe.hist.flat[binFlat(flatVal)]++
	spe.hist.diff[binDiff(diffVal)]++
}

// histogramThreshold returns the threshold (in raw feature units) that
// maximises the inter-class variance (Otsu's method applied to 1-D histograms).
// totalSamples is the sum of all counts in h.
// scale maps bucket index back to feature value: featureValue = (idx+0.5)/histBins * scale.
func histogramThreshold(h *[histBins]int, totalSamples int, scale float32) float32 {
	if totalSamples == 0 {
		return scale * 0.5
	}

	// compute cumulative sums of counts and bucket-weighted counts.
	var cumCount [histBins + 1]int
	var cumWeighted [histBins + 1]float64
	for i := 0; i < histBins; i++ {
		cumCount[i+1] = cumCount[i] + h[i]
		bucketCenter := (float64(i) + 0.5) / histBins
		cumWeighted[i+1] = cumWeighted[i] + float64(h[i])*bucketCenter
	}

	totalW := cumWeighted[histBins]
	n := float64(totalSamples)
	bestVar := -1.0
	bestIdx := histBins / 2

	for t := 1; t < histBins; t++ {
		n0 := float64(cumCount[t])
		n1 := n - n0
		if n0 == 0 || n1 == 0 {
			continue
		}
		mu0 := cumWeighted[t] / n0
		mu1 := (totalW - cumWeighted[t]) / n1
		v := (n0 / n) * (n1 / n) * (mu0 - mu1) * (mu0 - mu1)
		if v > bestVar {
			bestVar = v
			bestIdx = t
		}
	}

	// convert bucket boundary index back to feature scale.
	return float32(float64(bestIdx)/histBins) * scale
}

// adaptModel recomputes thresholds and weights from accumulated histograms.
// called once per histAdaptFrames frames.
func (spe *speechProbabilityEstimator) adaptModel() {
	n := spe.frameCount

	lrtThresh := histogramThreshold(&spe.hist.lrt, n, lrtMax)
	flatThresh := histogramThreshold(&spe.hist.flat, n, flatMax)
	diffThresh := histogramThreshold(&spe.hist.diff, n, diffMax)

	// clamp thresholds to sane ranges to avoid degenerate weights.
	if lrtThresh < 0.1 {
		lrtThresh = 0.1
	}
	if lrtThresh > lrtMax*0.8 {
		lrtThresh = lrtMax * 0.8
	}
	if flatThresh < 0.05 {
		flatThresh = 0.05
	}
	if flatThresh > 0.95 {
		flatThresh = 0.95
	}
	if diffThresh < 0.05 {
		diffThresh = 0.05
	}
	if diffThresh > 0.95 {
		diffThresh = 0.95
	}

	// weight each feature proportionally to how far its threshold deviates from
	// the centre of its range — a threshold near the centre implies good separation.
	// lrt weight is always dominant; flatness and diff are secondary.
	lrtSep := float32(math.Abs(float64(lrtThresh/lrtMax - 0.5)))
	flatSep := float32(math.Abs(float64(flatThresh/flatMax - 0.5)))
	diffSep := float32(math.Abs(float64(diffThresh/diffMax - 0.5)))

	// base weights: start from prior, blend toward data-driven separation scores.
	const alpha = 0.3 // blend factor: 0=keep prior, 1=full data-driven
	spe.prior.lrtThreshold = (1-alpha)*spe.prior.lrtThreshold + alpha*lrtThresh
	spe.prior.flatThreshold = (1-alpha)*spe.prior.flatThreshold + alpha*flatThresh
	spe.prior.diffThreshold = (1-alpha)*spe.prior.diffThreshold + alpha*diffThresh

	// weight update: lrt is kept at 1.0 (reference), others scaled by separation.
	// a feature with near-zero separation score contributes little.
	totalSep := lrtSep + flatSep + diffSep
	if totalSep > 0 {
		spe.prior.lrtWeight = 1.0
		spe.prior.flatWeight = (1-alpha)*spe.prior.flatWeight + alpha*(flatSep/totalSep)*1.5
		spe.prior.diffWeight = (1-alpha)*spe.prior.diffWeight + alpha*(diffSep/totalSep)*1.5
	}

	// clamp weights to reasonable bounds.
	if spe.prior.flatWeight > 1.5 {
		spe.prior.flatWeight = 1.5
	}
	if spe.prior.diffWeight > 1.5 {
		spe.prior.diffWeight = 1.5
	}

	// reset histograms for the next adaptation window.
	spe.hist = histograms{}
}

// Estimate computes per-bin speech probability using the current prior model.
// the indicator is a weighted combination of per-bin LRT deviation from
// threshold and frame-level spectral features, passed through a sigmoid.
// spectral flatness contributes negatively (flat = noise) and spectral
// difference contributes positively (changing spectrum = speech).
func (spe *speechProbabilityEstimator) Estimate(
	model *signalModel,
	noiseSpectrum [numFreqBins]float32,
	signalSpectrum [numFreqBins]float32,
	speechProb []float32,
) {
	spe.frameCount++

	// update histograms with this frame's aggregated features.
	spe.updateHistograms(model.avgLrt, model.spectralFlatness, model.spectralDiff)

	// adapt thresholds and weights after every histAdaptFrames frames.
	if spe.frameCount%histAdaptFrames == 0 {
		spe.adaptModel()
	}

	// flatness contribution: flat spectrum (high flatness) lowers speech probability.
	// we invert the sign so that high flatness -> negative indicator contribution.
	flatContrib := spe.prior.flatWeight * (spe.prior.flatThreshold - model.spectralFlatness)

	// diff contribution: large frame-to-frame spectral change raises speech probability.
	diffContrib := spe.prior.diffWeight * (model.spectralDiff - spe.prior.diffThreshold)

	for i := 0; i < numFreqBins; i++ {
		lrt := model.lrt[i]
		indicator := spe.prior.lrtWeight*(lrt-spe.prior.lrtThreshold) + flatContrib + diffContrib

		prob := float32(1.0 / (1.0 + math.Exp(-float64(indicator))))
		speechProb[i] = prob
	}
}

// signalModelEstimator computes per-frame and per-bin signal model features:
//   - per-bin LRT (likelihood ratio test) based on the Gaussian noise model
//   - spectral flatness (ratio of geometric to arithmetic mean of magnitude)
//   - spectral difference (normalised frame-to-frame magnitude change)
type signalModelEstimator struct {
	prevMagnitude [numFreqBins]float32
	frameCount    int
}

func newSignalModelEstimator() *signalModelEstimator {
	return &signalModelEstimator{}
}

// Update recomputes the signal model for the current frame.
// signalSpectrum and noiseSpectrum are log-power spectra (output of fastLog).
// the LRT per bin is computed as log(1 + linSNR) where linSNR = exp(logSNR) - 1,
// which is the standard log-likelihood ratio under a Gaussian speech model.
func (sme *signalModelEstimator) Update(
	signalSpectrum [numFreqBins]float32,
	noiseSpectrum [numFreqBins]float32,
	model *signalModel,
) {
	sme.frameCount++

	// per-bin LRT: log(1 + max(linSNR, 0)) where linSNR = exp(logSNR) - 1.
	// signalSpectrum[i] - noiseSpectrum[i] is log of the power ratio.
	// this formulation matches the WebRTC NS log-LRT used for histogram binning.
	var sumLrt float64
	for i := 0; i < numFreqBins; i++ {
		logSNR := signalSpectrum[i] - noiseSpectrum[i]
		linSNR := fastExp(logSNR) - 1.0
		if linSNR < 0 {
			linSNR = 0
		}
		lrt := fastLog(1.0 + linSNR)
		if lrt < 0 {
			lrt = 0
		}
		model.lrt[i] = lrt
		sumLrt += float64(lrt)
	}
	model.avgLrt = float32(sumLrt / numFreqBins)
	model.avgLogLrt = model.avgLrt // kept for external compatibility

	// spectral difference: normalised mean absolute frame-to-frame magnitude change.
	// magnitude is sqrt(power) = exp(0.5 * logPower).
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

	// spectral flatness: ratio of geometric mean to arithmetic mean of linear magnitude.
	// bins 1..numFreqBins-1 are used (skip DC at bin 0).
	// value in [0, 1]: 1 = perfectly flat (white noise), 0 = tonal / harmonic.
	var geoSum, ariSum float64
	for i := 1; i < numFreqBins; i++ {
		mag := fastExp(signalSpectrum[i])
		if mag < 1e-10 {
			mag = 1e-10
		}
		geoSum += float64(fastLog(mag))
		ariSum += float64(mag)
	}
	n := float64(numFreqBins - 1)
	geoMean := math.Exp(geoSum / n)
	ariMean := ariSum / n

	if ariMean > 0 {
		flat := float32(geoMean / ariMean)
		if flat > 1.0 {
			flat = 1.0
		}
		model.spectralFlatness = flat
	}
}
