package aec3

import "math"

// DelayEstimator estimates the echo-path delay in blocks using cross-correlation
// of down-sampled render and capture signals over a rolling history window.
type DelayEstimator struct {
	downSamplingFactor int
	numLags            int
	correlations       []float32
	renderHistory      []float32
	captureHistory     []float32
	historyPos         int
	historyLen         int
	historyMask        int
	estimatedDelay     int
}

// NewDelayEstimator creates a DelayEstimator sized according to config.
// the internal history buffer is rounded to the next power of two for efficient masking.
func NewDelayEstimator(config DelayConfig) *DelayEstimator {
	maxDelay := config.NumFilters * BlockSize / config.DownSamplingFactor
	minLen := maxDelay + BlockSize/config.DownSamplingFactor
	historyLen := nextPow2(minLen)
	return &DelayEstimator{
		downSamplingFactor: config.DownSamplingFactor,
		numLags:            maxDelay,
		correlations:       make([]float32, maxDelay),
		renderHistory:      make([]float32, historyLen),
		captureHistory:     make([]float32, historyLen),
		historyLen:         historyLen,
		historyMask:        historyLen - 1,
		estimatedDelay:     config.DefaultDelayBlocks,
	}
}

// Update ingests one render and one capture block, updates cross-correlations, and
// returns the estimated delay in blocks. both slices must be at least BlockSize FloatS16 samples.
func (de *DelayEstimator) Update(renderBlock, captureBlock []float32) int {
	dsLen := len(renderBlock) / de.downSamplingFactor
	mask := de.historyMask
	for i := 0; i < dsLen; i++ {
		idx := i * de.downSamplingFactor
		de.renderHistory[de.historyPos] = renderBlock[idx]
		de.captureHistory[de.historyPos] = captureBlock[idx]
		de.historyPos = (de.historyPos + 1) & mask
	}

	captureLen := dsLen
	bestCorr := float32(math.Inf(-1))
	bestLag := de.estimatedDelay

	maxLag := de.numLags
	if maxLag > de.historyLen-captureLen {
		maxLag = de.historyLen - captureLen
	}

	baseIdx := (de.historyPos - captureLen + de.historyLen) & mask

	for lag := 0; lag < maxLag; lag++ {
		var corr float32
		ci := baseIdx
		ri := (baseIdx - lag + de.historyLen) & mask
		for i := 0; i < captureLen; i++ {
			corr += de.captureHistory[ci] * de.renderHistory[ri]
			ci = (ci + 1) & mask
			ri = (ri + 1) & mask
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

// EstimatedDelay returns the most recently computed delay estimate in blocks.
func (de *DelayEstimator) EstimatedDelay() int {
	return de.estimatedDelay
}

// Reset clears all history buffers and correlation state.
func (de *DelayEstimator) Reset() {
	clear(de.correlations)
	clear(de.renderHistory)
	clear(de.captureHistory)
	de.historyPos = 0
}
