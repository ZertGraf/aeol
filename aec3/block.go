package aec3

// Block holds one 64-sample FloatS16 audio block for each frequency band and channel.
// band-channel indexing matches the splitting filter output; band 0 is 0-8 kHz.
type Block struct {
	data        [][]float32
	numBands    int
	numChannels int
}

// NewBlock allocates a Block with numBands frequency bands and numChannels channels.
// each band-channel slice has length BlockSize (64).
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

// View returns the FloatS16 slice for the given band and channel.
// the returned slice has length BlockSize and is backed by the block's internal storage.
func (b *Block) View(band, channel int) []float32 {
	return b.data[band*b.numChannels+channel]
}

// Clear zeroes all samples in the block.
func (b *Block) Clear() {
	for i := range b.data {
		clear(b.data[i])
	}
}

// NumBands returns the number of frequency bands in the block.
func (b *Block) NumBands() int { return b.numBands }

// NumChannels returns the number of audio channels in the block.
func (b *Block) NumChannels() int { return b.numChannels }

// CopyFrom copies all band-channel data from other into b.
// both blocks must have been created with the same numBands and numChannels.
func (b *Block) CopyFrom(other *Block) {
	for i := range b.data {
		copy(b.data[i], other.data[i])
	}
}

// FftData holds the real and imaginary parts of a half-spectrum produced by a
// 128-point real FFT, giving FFTSizeBy2Plus1 (65) independent complex bins.
type FftData struct {
	Re [FFTSizeBy2Plus1]float32
	Im [FFTSizeBy2Plus1]float32
}

// Clear zeroes both the Re and Im arrays.
func (f *FftData) Clear() {
	clear(f.Re[:])
	clear(f.Im[:])
}

// CopyFrom copies Re and Im arrays from other into f.
func (f *FftData) CopyFrom(other *FftData) {
	copy(f.Re[:], other.Re[:])
	copy(f.Im[:], other.Im[:])
}

// Spectrum writes the power spectrum Re[k]^2 + Im[k]^2 into out.
// out must have length at least FFTSizeBy2Plus1 (65).
func (f *FftData) Spectrum(out []float32) {
	for i := 0; i < FFTSizeBy2Plus1; i++ {
		out[i] = f.Re[i]*f.Re[i] + f.Im[i]*f.Im[i]
	}
}
