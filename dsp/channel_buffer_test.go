package dsp

import (
	"testing"
)

func TestChannelBuffer(t *testing.T) {
	cb := NewChannelBuffer(4, 2, 3) // frames=4, ch=2, bands=3
	
	if cb.NumFrames() != 4 {
		t.Errorf("expected 4 frames, got %v", cb.NumFrames())
	}
	if cb.NumChannels() != 2 {
		t.Errorf("expected 2 channels, got %v", cb.NumChannels())
	}
	if cb.NumBands() != 3 {
		t.Errorf("expected 3 bands, got %v", cb.NumBands())
	}

	cb.Slice()[0] = 99.0
	
	ch0 := cb.Channel(0)
	if len(ch0) != 4 {
		t.Errorf("expected channel length 4, got %v", len(ch0))
	}
	if ch0[0] != 99.0 {
		t.Errorf("expected 99.0 in ch0")
	}

	band1 := cb.Band(0, 1)
	if len(band1) != 4 {
		t.Errorf("expected band length 4, got %v", len(band1))
	}
	
	cb.Clear()
	if cb.Slice()[0] != 0 {
		t.Errorf("expected 0 after clear")
	}
}
