// Package bands provides frequency band splitting for use with stages
// that operate at 16 kHz only (ns.Suppressor, aec3.EchoCanceller3).
//
// At sample rates above 16 kHz the input must be split into frequency
// bands before processing: the lower band (0–8 kHz, 160 samples) goes
// through the 16 kHz stage, upper bands are passed through or
// gain-matched. After processing, bands are merged back into
// a full-rate frame.
//
//	sp := bands.New(48000) // 3 bands for 48 kHz
//	lower, upper := sp.Split(frame480)
//	suppressor.Process(lower)
//	for i, ub := range upper {
//		suppressor.ProcessUpperBand(ub, i)
//	}
//	sp.Merge(lower, upper, frame480)
//
// At 16 kHz Split returns the frame itself and nil upper bands;
// Merge is a no-op copy.
//
// Instances are not safe for concurrent use; synchronization is the caller's responsibility.
package bands

import "sonora/dsp"

// Splitter splits a single-channel audio frame into frequency bands
// and merges them back. One Splitter per channel.
type Splitter struct {
	numBands int
	frameLen int
	bandLen  int
	filter   *dsp.SplittingFilter
	bandBuf  [][]float32
	fullBuf  []float32
}

// New creates a Splitter for the given sample rate.
// Supported rates: 16000, 32000, 48000. Returns nil for unsupported rates.
func New(sampleRateHz uint32) *Splitter {
	switch sampleRateHz {
	case 16000, 32000, 48000:
	default:
		return nil
	}
	n := numBands(sampleRateHz)
	frameLen := int(sampleRateHz) / 100
	bandLen := frameLen / n

	s := &Splitter{
		numBands: n,
		frameLen: frameLen,
		bandLen:  bandLen,
	}

	if n > 1 {
		s.filter = dsp.NewSplittingFilter(1, n)
		s.bandBuf = make([][]float32, n)
		for i := 0; i < n; i++ {
			s.bandBuf[i] = make([]float32, bandLen)
		}
		s.fullBuf = make([]float32, frameLen)
	}

	return s
}

// Bands returns the number of frequency bands for this splitter.
func (s *Splitter) Bands() int {
	return s.numBands
}

// BandLength returns the number of samples per band (160 for all supported rates).
func (s *Splitter) BandLength() int {
	return s.bandLen
}

// FrameLength returns the expected full-rate frame length.
func (s *Splitter) FrameLength() int {
	return s.frameLen
}

// Split decomposes frame into frequency bands via the QMF analysis filter.
// Returns the lower band slice (160 samples, suitable for NS/AEC3) and
// upper band slices. At 16 kHz, lower is the frame itself and upper is nil.
//
// The returned slices are owned by the Splitter and valid until the next
// call to Split.
func (s *Splitter) Split(frame []float32) (lower []float32, upper [][]float32) {
	if s.numBands == 1 {
		return frame, nil
	}

	copy(s.fullBuf, frame[:s.frameLen])
	s.filter.Analysis(
		[][]float32{s.fullBuf},
		[][][]float32{s.bandBuf},
	)

	return s.bandBuf[0], s.bandBuf[1:]
}

// Merge reconstructs a full-rate frame from processed bands via the QMF
// synthesis filter. At 16 kHz this copies lower into frame.
func (s *Splitter) Merge(lower []float32, upper [][]float32, frame []float32) {
	if s.numBands == 1 {
		copy(frame, lower[:s.bandLen])
		return
	}

	copy(s.bandBuf[0], lower[:s.bandLen])
	for i, ub := range upper {
		copy(s.bandBuf[1+i], ub[:s.bandLen])
	}

	s.filter.Synthesis(
		[][][]float32{s.bandBuf},
		[][]float32{frame[:s.frameLen]},
	)
}

// Reset clears the filter state.
func (s *Splitter) Reset() {
	if s.filter != nil {
		s.filter = dsp.NewSplittingFilter(1, s.numBands)
	}
	for _, b := range s.bandBuf {
		clear(b)
	}
}

func numBands(sampleRate uint32) int {
	switch {
	case sampleRate <= 16000:
		return 1
	case sampleRate <= 32000:
		return 2
	default:
		return 3
	}
}
