package dsp

import (
	"math"
	"testing"
)

func TestAudioUtilConversions(t *testing.T) {
	if v := FloatS16ToS16(32768.5); v != MaxS16 {
		t.Errorf("expected %v, got %v", MaxS16, v)
	}
	if v := FloatS16ToS16(-32769.5); v != MinS16 {
		t.Errorf("expected %v, got %v", MinS16, v)
	}
	if v := FloatS16ToS16(1234.0); v != 1234 {
		t.Errorf("expected %v, got %v", 1234, v)
	}
	if v := FloatS16ToS16(-1234.0); v != -1234 {
		t.Errorf("expected %v, got %v", -1234, v)
	}

	if v := S16ToFloatS16(1234); v != 1234.0 {
		t.Errorf("expected %v, got %v", 1234.0, v)
	}

	norm := S16ToFloatNorm(16384)
	if norm != 0.5 {
		t.Errorf("expected 0.5, got %v", norm)
	}
	if v := FloatNormToS16(0.5); v != 16384 {
		t.Errorf("expected 16384, got %v", v)
	}

	if db := FloatS16ToDbfs(0); db != -100 {
		t.Errorf("expected -100, got %v", db)
	}
	if db := FloatS16ToDbfs(32768.0); math.Abs(float64(db)) > 1e-4 {
		t.Errorf("expected 0, got %v", db)
	}
}

func TestAudioUtilInterleavingS16(t *testing.T) {
	planarNorm := [][]float32{
		{0.5, 0.25},
		{-0.5, -0.25},
	}
	interleavedS16 := make([]int16, 4)
	InterleaveToS16(planarNorm, 2, interleavedS16)
	
	outPlanarNorm := [][]float32{make([]float32, 2), make([]float32, 2)}
	DeinterleaveS16(interleavedS16, 2, outPlanarNorm)
	for ch := 0; ch < 2; ch++ {
		for i := 0; i < 2; i++ {
			if math.Abs(float64(outPlanarNorm[ch][i]-planarNorm[ch][i])) > 1e-4 {
				t.Errorf("expected %v, got %v", planarNorm, outPlanarNorm)
			}
		}
	}
	InterleaveToS16([][]float32{}, 0, []int16{})
}

func TestAudioUtilUpmix(t *testing.T) {
	singleChannel := [][]float32{{10.0, 20.0}}
	mono2 := make([]float32, 2)
	DownmixToMono(singleChannel, mono2)

	outChannels := [][]float32{make([]float32, 2), make([]float32, 2)}
	UpmixFromMono(mono2, outChannels)
	if outChannels[0][0] != 10.0 || outChannels[1][1] != 20.0 {
		t.Errorf("expected upmix to match mono")
	}
}
