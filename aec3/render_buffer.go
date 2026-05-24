package aec3

// RenderBuffer is a circular buffer of frequency-domain render (far-end) blocks.
// it stores FftData frames indexed by delay, enabling the adaptive filter to access
// past render spectra up to filterLength blocks back.
type RenderBuffer struct {
	blocks   []FftData
	writePos int
	readPos  int
	size     int
	mask     int
}

// NewRenderBuffer creates a RenderBuffer with capacity rounded up to the next power of two
// that is at least numBlocks.
func NewRenderBuffer(numBlocks int) *RenderBuffer {
	size := nextPow2(numBlocks)
	return &RenderBuffer{
		blocks: make([]FftData, size),
		size:   size,
		mask:   size - 1,
	}
}

// Insert appends a copy of data as the newest render spectrum.
func (rb *RenderBuffer) Insert(data *FftData) {
	rb.blocks[rb.writePos].CopyFrom(data)
	rb.writePos = (rb.writePos + 1) & rb.mask
}

// Block returns a pointer to the render spectrum that is delay blocks in the past.
// delay 0 is the most recently inserted block; must be less than the buffer capacity.
func (rb *RenderBuffer) Block(delay int) *FftData {
	idx := (rb.writePos - 1 - delay + rb.size) & rb.mask
	return &rb.blocks[idx]
}

// Size returns the total capacity of the buffer in blocks.
func (rb *RenderBuffer) Size() int {
	return rb.size
}

// Reset clears all stored spectra and resets the write position.
func (rb *RenderBuffer) Reset() {
	for i := range rb.blocks {
		rb.blocks[i].Clear()
	}
	rb.writePos = 0
	rb.readPos = 0
}

func nextPow2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}
