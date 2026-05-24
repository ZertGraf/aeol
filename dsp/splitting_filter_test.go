package dsp

import (
	"math"
	"testing"
)

func TestSplittingFilter2Bands(t *testing.T) {
	numChannels := 1
	numBands := 2
	sf := NewSplittingFilter(numChannels, numBands)

	data := [][]float32{make([]float32, TwoBandFilterSamplesPerFrame)}
	for i := range data[0] {
		data[0][i] = float32(math.Sin(float64(i) * 0.1))
	}

	bands := [][][]float32{
		{
			make([]float32, SamplesPerBand),
			make([]float32, SamplesPerBand),
		},
	}

	sf.Analysis(data, bands)

	// Output buffer
	outData := [][]float32{make([]float32, TwoBandFilterSamplesPerFrame)}
	sf.Synthesis(bands, outData)

	// The signal should be approximately reconstructed
	var diff float32
	for i := range data[0] {
		d := outData[0][i] - data[0][i]
		if d < 0 {
			d = -d
		}
		diff += d
	}
	if diff/float32(len(data[0])) > 0.5 { // rough check
		t.Errorf("2-band synthesis diff too high: %v", diff)
	}
}

func TestSplittingFilter3Bands(t *testing.T) {
	numChannels := 1
	numBands := 3
	sf := NewSplittingFilter(numChannels, numBands)

	// broadband signal spanning many frames (sum of harmonics)
	numFrames := 30
	signal := make([]float32, FullBandSize*numFrames)
	for i := range signal {
		x := float64(i)
		signal[i] = float32(0.3*math.Sin(x*0.01) + 0.3*math.Sin(x*0.05) +
			0.2*math.Sin(x*0.15) + 0.2*math.Sin(x*0.3))
	}

	data := [][]float32{make([]float32, FullBandSize)}
	bands := [][][]float32{
		{
			make([]float32, SplitBandSize),
			make([]float32, SplitBandSize),
			make([]float32, SplitBandSize),
		},
	}
	outData := [][]float32{make([]float32, FullBandSize)}
	reconstructed := make([]float32, FullBandSize*numFrames)

	for frame := 0; frame < numFrames; frame++ {
		copy(data[0], signal[frame*FullBandSize:(frame+1)*FullBandSize])
		sf.Analysis(data, bands)
		sf.Synthesis(bands, outData)
		copy(reconstructed[frame*FullBandSize:(frame+1)*FullBandSize], outData[0])
	}

	// polyphase filter bank introduces group delay (~51 samples);
	// compare with delay compensation after convergence
	delay := 51
	start := 10 * FullBandSize
	end := 25 * FullBandSize
	var diff float64
	var count int
	for i := start; i < end; i++ {
		if i-delay >= 0 {
			d := float64(reconstructed[i]) - float64(signal[i-delay])
			if d < 0 {
				d = -d
			}
			diff += d
			count++
		}
	}
	avg := diff / float64(count)
	if avg > 0.2 {
		t.Errorf("3-band synthesis error too high (delay-compensated): avg=%.6f", avg)
	}
}

func TestThreeBandFilterBank(t *testing.T) {
	fb := NewThreeBandFilterBank()
	in := make([]float32, FullBandSize)
	for i := range in {
		in[i] = 1.0
	}
	outBands := [][]float32{
		make([]float32, SplitBandSize),
		make([]float32, SplitBandSize),
		make([]float32, SplitBandSize),
	}
	fb.Analysis(in, outBands)

	outData := make([]float32, FullBandSize)
	fb.Synthesis(outBands, outData)
}
