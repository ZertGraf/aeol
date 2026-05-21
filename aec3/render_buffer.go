package aec3

type RenderBuffer struct {
	blocks   []*FftData
	writePos int
	readPos  int
	size     int
}

func NewRenderBuffer(numBlocks int) *RenderBuffer {
	blocks := make([]*FftData, numBlocks)
	for i := range blocks {
		blocks[i] = &FftData{}
	}
	return &RenderBuffer{
		blocks: blocks,
		size:   numBlocks,
	}
}

func (rb *RenderBuffer) Insert(data *FftData) {
	rb.blocks[rb.writePos].CopyFrom(data)
	rb.writePos = (rb.writePos + 1) % rb.size
}

func (rb *RenderBuffer) Block(delay int) *FftData {
	idx := (rb.writePos - 1 - delay + rb.size*2) % rb.size
	return rb.blocks[idx]
}

func (rb *RenderBuffer) Size() int {
	return rb.size
}

func (rb *RenderBuffer) Reset() {
	for _, b := range rb.blocks {
		b.Clear()
	}
	rb.writePos = 0
	rb.readPos = 0
}
