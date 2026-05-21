package ns

import "math"

type wienerFilter struct {
	gains        [numFreqBins]float32
	overallScale float32
}

func newWienerFilter() *wienerFilter {
	wf := &wienerFilter{overallScale: 1.0}
	for i := range wf.gains {
		wf.gains[i] = 1.0
	}
	return wf
}

func (wf *wienerFilter) Update(
	signalSpectrum [numFreqBins]float32,
	noiseSpectrum [numFreqBins]float32,
	speechProb [numFreqBins]float32,
	params suppressionParams,
) {
	for i := 0; i < numFreqBins; i++ {
		snr := signalSpectrum[i] - noiseSpectrum[i]
		if snr < 0 {
			snr = 0
		}

		priori := snr * speechProb[i]
		gain := priori / (priori + params.overSubtractionFactor)

		if gain < 1.0/params.minOverDrive {
			gain = 1.0 / params.minOverDrive
		}

		wf.gains[i] = 0.5*wf.gains[i] + 0.5*gain
	}

	var sumGain float64
	for i := 0; i < numFreqBins; i++ {
		sumGain += float64(wf.gains[i])
	}
	avgGain := float32(sumGain / numFreqBins)

	wf.overallScale = 1.0
	if avgGain < 0.5 {
		wf.overallScale = float32(2.0 * math.Sqrt(float64(avgGain)))
		if wf.overallScale < 0.2 {
			wf.overallScale = 0.2
		}
	}
}

func (wf *wienerFilter) Apply(re, im []float32) {
	for i := 0; i < numFreqBins && i < len(re) && i < len(im); i++ {
		g := wf.gains[i] * wf.overallScale
		re[i] *= g
		im[i] *= g
	}
}

func (wf *wienerFilter) Gains() [numFreqBins]float32 {
	return wf.gains
}
