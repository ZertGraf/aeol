package aec3

import "math"

type DelayEstimator struct {
	downSamplingFactor int
	numLags            int
	correlations       []float32
	renderHistory      []float32
	captureHistory     []float32
	historyPos         int
	historyLen         int
	estimatedDelay     int
	confidence         float32
}

func NewDelayEstimator(config DelayConfig) *DelayEstimator {
	maxDelay := config.NumFilters * BlockSize / config.DownSamplingFactor
	historyLen := maxDelay + BlockSize/config.DownSamplingFactor
	return &DelayEstimator{
		downSamplingFactor: config.DownSamplingFactor,
		numLags:            maxDelay,
		correlations:       make([]float32, maxDelay),
		renderHistory:      make([]float32, historyLen),
		captureHistory:     make([]float32, historyLen),
		historyLen:         historyLen,
		estimatedDelay:     config.DefaultDelayBlocks,
	}
}

func (de *DelayEstimator) Update(renderBlock, captureBlock []float32) int {
	dsLen := len(renderBlock) / de.downSamplingFactor
	for i := 0; i < dsLen; i++ {
		idx := i * de.downSamplingFactor
		de.renderHistory[de.historyPos] = renderBlock[idx]
		de.captureHistory[de.historyPos] = captureBlock[idx]
		de.historyPos = (de.historyPos + 1) % de.historyLen
	}

	captureLen := dsLen
	bestCorr := float32(math.Inf(-1))
	bestLag := de.estimatedDelay

	for lag := 0; lag < de.numLags && lag < de.historyLen-captureLen; lag++ {
		var corr float32
		for i := 0; i < captureLen; i++ {
			captIdx := (de.historyPos - captureLen + i + de.historyLen) % de.historyLen
			rendIdx := (captIdx - lag + de.historyLen) % de.historyLen
			corr += de.captureHistory[captIdx] * de.renderHistory[rendIdx]
		}
		de.correlations[lag] = 0.9*de.correlations[lag] + 0.1*corr
		if de.correlations[lag] > bestCorr {
			bestCorr = de.correlations[lag]
			bestLag = lag
		}
	}

	de.estimatedDelay = bestLag * de.downSamplingFactor / BlockSize
	return de.estimatedDelay
}

func (de *DelayEstimator) EstimatedDelay() int {
	return de.estimatedDelay
}

func (de *DelayEstimator) Confidence() float32 {
	return de.confidence
}

func (de *DelayEstimator) Reset() {
	clear(de.correlations)
	clear(de.renderHistory)
	clear(de.captureHistory)
	de.historyPos = 0
	de.confidence = 0
}
