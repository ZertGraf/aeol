package aec3

type Block struct {
	data        [][]float32
	numBands    int
	numChannels int
}

func NewBlock(numBands, numChannels int) *Block {
	data := make([][]float32, numBands*numChannels)
	for i := range data {
		data[i] = make([]float32, BlockSize)
	}
	return &Block{
		data:        data,
		numBands:    numBands,
		numChannels: numChannels,
	}
}

func (b *Block) View(band, channel int) []float32 {
	return b.data[band*b.numChannels+channel]
}

func (b *Block) Clear() {
	for i := range b.data {
		clear(b.data[i])
	}
}

func (b *Block) NumBands() int    { return b.numBands }
func (b *Block) NumChannels() int { return b.numChannels }

func (b *Block) CopyFrom(other *Block) {
	for i := range b.data {
		copy(b.data[i], other.data[i])
	}
}

type FftData struct {
	Re [FFTSizeBy2Plus1]float32
	Im [FFTSizeBy2Plus1]float32
}

func (f *FftData) Clear() {
	clear(f.Re[:])
	clear(f.Im[:])
}

func (f *FftData) CopyFrom(other *FftData) {
	copy(f.Re[:], other.Re[:])
	copy(f.Im[:], other.Im[:])
}

func (f *FftData) Spectrum(out []float32) {
	for i := 0; i < FFTSizeBy2Plus1; i++ {
		out[i] = f.Re[i]*f.Re[i] + f.Im[i]*f.Im[i]
	}
}
