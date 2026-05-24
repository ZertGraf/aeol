package ns

type signalModel struct {
	lrt              [numFreqBins]float32
	spectralFlatness float32
	spectralDiff     float32
	avgLogLrt        float32
	// avgLrt is the mean per-bin LRT value used for histogram binning.
	// unlike avgLogLrt (which is summed SNR), this is the mean of lrt[] values.
	avgLrt float32
}
