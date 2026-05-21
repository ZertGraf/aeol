package aec3

type FrameBlocker struct {
	buffer    []float32
	bufferPos int
	numBands  int
}

func NewFrameBlocker(numBands int) *FrameBlocker {
	return &FrameBlocker{
		buffer:   make([]float32, BlockSize*numBands),
		numBands: numBands,
	}
}

func (fb *FrameBlocker) InsertSubFrame(subFrame []float32) {
	n := min(len(subFrame), len(fb.buffer)-fb.bufferPos)
	copy(fb.buffer[fb.bufferPos:], subFrame[:n])
	fb.bufferPos += n
}

func (fb *FrameBlocker) IsBlockAvailable() bool {
	return fb.bufferPos >= BlockSize
}

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

func (fb *FrameBlocker) Reset() {
	fb.bufferPos = 0
	clear(fb.buffer)
}

type BlockFramer struct {
	buffer    []float32
	bufferPos int
	numBands  int
}

func NewBlockFramer(numBands int) *BlockFramer {
	return &BlockFramer{
		buffer:   make([]float32, SubFrameLength*numBands),
		numBands: numBands,
	}
}

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

func (bf *BlockFramer) IsFrameAvailable() bool {
	return bf.bufferPos >= SubFrameLength
}

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

func (bf *BlockFramer) Reset() {
	bf.bufferPos = 0
	clear(bf.buffer)
}
