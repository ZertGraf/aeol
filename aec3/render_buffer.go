package aec3

type RenderBuffer struct {
	blocks   []FftData
	writePos int
	readPos  int
	size     int
	mask     int
}

func NewRenderBuffer(numBlocks int) *RenderBuffer {
	size := nextPow2(numBlocks)
	return &RenderBuffer{
		blocks: make([]FftData, size),
		size:   size,
		mask:   size - 1,
	}
}

func (rb *RenderBuffer) Insert(data *FftData) {
	rb.blocks[rb.writePos].CopyFrom(data)
	rb.writePos = (rb.writePos + 1) & rb.mask
}

func (rb *RenderBuffer) Block(delay int) *FftData {
	idx := (rb.writePos - 1 - delay + rb.size) & rb.mask
	return &rb.blocks[idx]
}

func (rb *RenderBuffer) Size() int {
	return rb.size
}

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
