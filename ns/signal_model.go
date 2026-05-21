package ns

type signalModel struct {
	lrt             [numFreqBins]float32
	spectralFlatness float32
	spectralDiff     float32
	avgLogLrt        float32
}
