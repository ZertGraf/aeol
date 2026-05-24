package dsp

import (
	"math"
	"testing"
)

func TestSincResampler(t *testing.T) {
	// Simple test to ensure SincResampler instantiates and processes frames
	requestFrames := 160
	ioRatio := 0.5 // downsample
	resampler := NewSincResampler(ioRatio, requestFrames)

	src := make([]float32, 320)
	for i := range src {
		src[i] = float32(math.Sin(2 * math.Pi * 440 * float64(i) / 32000))
	}
	dst := make([]float32, 160)

	produced := resampler.Resample(src, dst)
	if produced <= 0 {
		t.Errorf("expected > 0 samples produced, got %v", produced)
	}

	resampler.Reset()
}

func TestPushResampler(t *testing.T) {
	srcRate := 48000
	dstRate := 16000
	numChannels := 2

	pr := NewPushResampler(srcRate, dstRate, numChannels)
	if pr.OutputFrames() != 160 {
		t.Errorf("expected 160 frames, got %v", pr.OutputFrames())
	}

	src := [][]float32{
		make([]float32, 480),
		make([]float32, 480),
	}
	for i := range src[0] {
		src[0][i] = 0.5
		src[1][i] = -0.5
	}
	dst := [][]float32{
		make([]float32, 160),
		make([]float32, 160),
	}

	pr.Resample(src, dst)

	// Check same rate passthrough
	prSame := NewPushResampler(16000, 16000, 2)
	srcSame := [][]float32{
		make([]float32, 160),
		make([]float32, 160),
	}
	dstSame := [][]float32{
		make([]float32, 160),
		make([]float32, 160),
	}
	srcSame[0][0] = 1.0
	prSame.Resample(srcSame, dstSame)
	if dstSame[0][0] != 1.0 {
		t.Errorf("expected 1.0, got %v", dstSame[0][0])
	}
}
