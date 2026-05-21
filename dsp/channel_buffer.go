package dsp

type ChannelBuffer struct {
	data        []float32
	numChannels int
	numBands    int
	numFrames   int
}

func NewChannelBuffer(numFrames, numChannels, numBands int) *ChannelBuffer {
	total := numFrames * numChannels * numBands
	return &ChannelBuffer{
		data:        make([]float32, total),
		numChannels: numChannels,
		numBands:    numBands,
		numFrames:   numFrames,
	}
}

func (cb *ChannelBuffer) Channel(ch int) []float32 {
	start := ch * cb.numBands * cb.numFrames
	return cb.data[start : start+cb.numFrames]
}

func (cb *ChannelBuffer) Band(ch, band int) []float32 {
	start := (ch*cb.numBands + band) * cb.numFrames
	return cb.data[start : start+cb.numFrames]
}

func (cb *ChannelBuffer) NumChannels() int { return cb.numChannels }
func (cb *ChannelBuffer) NumBands() int    { return cb.numBands }
func (cb *ChannelBuffer) NumFrames() int   { return cb.numFrames }

func (cb *ChannelBuffer) Clear() {
	clear(cb.data)
}

func (cb *ChannelBuffer) Slice() []float32 {
	return cb.data
}
