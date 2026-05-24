package sonora

import "sonora/hpf"

// HighPassFilter wraps one hpf.Filter per channel.
// Kept for use by benchmark tests that operate at the sonora package level.
type HighPassFilter struct {
	filters []*hpf.Filter
}

// NewHighPassFilter creates a HighPassFilter with numChannels independent
// single-channel filters tuned for sampleRate.
func NewHighPassFilter(sampleRate uint32, numChannels int) *HighPassFilter {
	filters := make([]*hpf.Filter, numChannels)
	for ch := range filters {
		filters[ch] = hpf.New(sampleRate)
	}
	return &HighPassFilter{filters: filters}
}

// Process applies the high-pass filter in-place to each channel.
// len(channels) may be less than the number of filters; extra filters are skipped.
func (h *HighPassFilter) Process(channels [][]float32) {
	for ch := 0; ch < len(h.filters) && ch < len(channels); ch++ {
		h.filters[ch].Process(channels[ch])
	}
}

// Reset clears the state of all channel filters.
func (h *HighPassFilter) Reset() {
	for _, f := range h.filters {
		f.Reset()
	}
}
