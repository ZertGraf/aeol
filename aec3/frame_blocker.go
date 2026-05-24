package aec3

// FrameBlocker accumulates incoming sub-frame audio and emits complete 64-sample blocks.
// use InsertSubFrame to feed data and ExtractBlock once IsBlockAvailable returns true.
type FrameBlocker struct {
	buffer    []float32
	bufferPos int
	numBands  int
}

// NewFrameBlocker creates a FrameBlocker for audio with the given number of frequency bands.
func NewFrameBlocker(numBands int) *FrameBlocker {
	return &FrameBlocker{
		buffer:   make([]float32, BlockSize*numBands),
		numBands: numBands,
	}
}

// InsertSubFrame appends subFrame samples into the internal buffer.
// subFrame contains interleaved FloatS16 samples for all bands.
func (fb *FrameBlocker) InsertSubFrame(subFrame []float32) {
	n := min(len(subFrame), len(fb.buffer)-fb.bufferPos)
	copy(fb.buffer[fb.bufferPos:], subFrame[:n])
	fb.bufferPos += n
}

// IsBlockAvailable reports whether a full BlockSize (64) block is ready to extract.
func (fb *FrameBlocker) IsBlockAvailable() bool {
	return fb.bufferPos >= BlockSize
}

// ExtractBlock copies one ready block into block and advances the internal read position.
// does nothing if IsBlockAvailable returns false.
func (fb *FrameBlocker) ExtractBlock(block *Block) {
	if !fb.IsBlockAvailable() {
		return
	}
	for band := 0; band < block.NumBands() && band < fb.numBands; band++ {
		for ch := 0; ch < block.NumChannels(); ch++ {
			view := block.View(band, ch)
			start := band * BlockSize
			copy(view, fb.buffer[start:start+BlockSize])
		}
	}

	remaining := fb.bufferPos - BlockSize
	if remaining > 0 {
		copy(fb.buffer, fb.buffer[BlockSize:fb.bufferPos])
	}
	fb.bufferPos = remaining
}

// Reset clears the internal buffer and resets the write position.
func (fb *FrameBlocker) Reset() {
	fb.bufferPos = 0
	clear(fb.buffer)
}

// BlockFramer collects 64-sample blocks and assembles them into SubFrameLength (80-sample) frames.
// use InsertBlock to feed data and ExtractFrame once IsFrameAvailable returns true.
type BlockFramer struct {
	buffer    []float32
	bufferPos int
	numBands  int
}

// NewBlockFramer creates a BlockFramer for audio with the given number of frequency bands.
func NewBlockFramer(numBands int) *BlockFramer {
	return &BlockFramer{
		buffer:   make([]float32, SubFrameLength*numBands),
		numBands: numBands,
	}
}

// InsertBlock appends the samples from block into the internal buffer.
func (bf *BlockFramer) InsertBlock(block *Block) {
	for band := 0; band < block.NumBands() && band < bf.numBands; band++ {
		for ch := 0; ch < block.NumChannels(); ch++ {
			view := block.View(band, ch)
			n := min(len(view), len(bf.buffer)-bf.bufferPos)
			copy(bf.buffer[bf.bufferPos:], view[:n])
			bf.bufferPos += n
		}
	}
}

// IsFrameAvailable reports whether a full SubFrameLength (80-sample) frame is ready to extract.
func (bf *BlockFramer) IsFrameAvailable() bool {
	return bf.bufferPos >= SubFrameLength
}

// ExtractFrame copies SubFrameLength (80) FloatS16 samples into frame and advances the read position.
// does nothing if IsFrameAvailable returns false; frame must have capacity >= SubFrameLength.
func (bf *BlockFramer) ExtractFrame(frame []float32) {
	if !bf.IsFrameAvailable() {
		return
	}
	copy(frame, bf.buffer[:SubFrameLength])
	remaining := bf.bufferPos - SubFrameLength
	if remaining > 0 {
		copy(bf.buffer, bf.buffer[SubFrameLength:bf.bufferPos])
	}
	bf.bufferPos = remaining
}

// Reset clears the internal buffer and resets the write position.
func (bf *BlockFramer) Reset() {
	bf.bufferPos = 0
	clear(bf.buffer)
}
