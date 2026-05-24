package dsp

import (
	"math"

	"aeol/simd"
)

const (
	sincKernelSize       = 32
	sincResamplerOffsets = 32
	sincBufferSize       = sincKernelSize * (sincResamplerOffsets + 1)
)

type SincResampler struct {
	ioRatio     float64
	virtualPos  float64
	buffer      []float32
	kernelTable [sincBufferSize]float64
	backend     simd.Backend
}

func NewSincResampler(ioRatio float64, requestFrames int) *SincResampler {
	s := &SincResampler{
		ioRatio: ioRatio,
		backend: simd.Default(),
	}
	srcFrames := int(math.Ceil(float64(requestFrames) * ioRatio))
	s.buffer = make([]float32, sincKernelSize, sincKernelSize+srcFrames+10)
	s.initKernelTable()
	return s
}

func (s *SincResampler) initKernelTable() {
	sincScaleFactor := 1.0
	if s.ioRatio > 1.0 {
		sincScaleFactor = 1.0 / s.ioRatio
	}

	for offsetIdx := 0; offsetIdx <= sincResamplerOffsets; offsetIdx++ {
		subsampleOffset := float64(offsetIdx) / float64(sincResamplerOffsets)
		for i := 0; i < sincKernelSize; i++ {
			idx := offsetIdx*sincKernelSize + i
			n := float64(i) - float64(sincKernelSize/2) + 1.0 - subsampleOffset
			val := sinc(sincScaleFactor * n)
			val *= sincScaleFactor
			val *= blackmanWindow(n, sincKernelSize)
			s.kernelTable[idx] = val
		}
	}
}

func (s *SincResampler) Resample(src []float32, dst []float32) int {
	s.buffer = append(s.buffer, src...)

	dstLen := len(dst)
	dstIdx := 0
	bufLen := len(s.buffer)

	for dstIdx < dstLen {
		intPos := int(s.virtualPos)
		frac := s.virtualPos - float64(intPos)

		inputStart := intPos
		inputEnd := inputStart + sincKernelSize
		if inputEnd > bufLen {
			break
		}

		offsetIdx := int(frac * float64(sincResamplerOffsets))
		if offsetIdx >= sincResamplerOffsets {
			offsetIdx = sincResamplerOffsets - 1
		}

		k1Start := offsetIdx * sincKernelSize
		k2Start := (offsetIdx + 1) * sincKernelSize
		interpFactor := frac*float64(sincResamplerOffsets) - float64(offsetIdx)

		input := s.buffer[inputStart:inputEnd]
		k1 := s.kernelTable[k1Start : k1Start+sincKernelSize]
		k2 := s.kernelTable[k2Start : k2Start+sincKernelSize]

		dst[dstIdx] = s.backend.ConvolveSinc(input, k1, k2, interpFactor)
		dstIdx++

		s.virtualPos += s.ioRatio
	}

	intPos := int(s.virtualPos)
	if intPos > 0 {
		if intPos < len(s.buffer) {
			copy(s.buffer, s.buffer[intPos:])
			s.buffer = s.buffer[:len(s.buffer)-intPos]
		} else {
			s.buffer = s.buffer[:0]
		}
		s.virtualPos -= float64(intPos)
	}

	return dstIdx
}

func (s *SincResampler) Reset() {
	s.virtualPos = 0
	if len(s.buffer) >= sincKernelSize {
		s.buffer = s.buffer[:sincKernelSize]
	} else {
		s.buffer = make([]float32, sincKernelSize)
	}
	clear(s.buffer)
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-10 {
		return 1.0
	}
	px := math.Pi * x
	return math.Sin(px) / px
}

func blackmanWindow(n float64, size int) float64 {
	x := n / float64(size)
	return 0.42 - 0.5*math.Cos(2*math.Pi*x) + 0.08*math.Cos(4*math.Pi*x)
}

type PushResampler struct {
	sincs       []*SincResampler
	srcRate     int
	dstRate     int
	numChannels int
	srcBuf      [][]float32
	dstBuf      [][]float32
}

func NewPushResampler(srcRate, dstRate, numChannels int) *PushResampler {
	srcFrames := srcRate / 100
	dstFrames := dstRate / 100

	pr := &PushResampler{
		srcRate:     srcRate,
		dstRate:     dstRate,
		numChannels: numChannels,
		srcBuf:      make([][]float32, numChannels),
		dstBuf:      make([][]float32, numChannels),
	}

	if srcRate != dstRate {
		ioRatio := float64(srcRate) / float64(dstRate)
		pr.sincs = make([]*SincResampler, numChannels)
		for ch := 0; ch < numChannels; ch++ {
			pr.sincs[ch] = NewSincResampler(ioRatio, dstFrames)
		}
	}

	for ch := 0; ch < numChannels; ch++ {
		pr.srcBuf[ch] = make([]float32, srcFrames)
		pr.dstBuf[ch] = make([]float32, dstFrames)
	}

	return pr
}

func (pr *PushResampler) Resample(src [][]float32, dst [][]float32) {
	if pr.sincs == nil {
		for ch := 0; ch < pr.numChannels; ch++ {
			copy(dst[ch], src[ch])
		}
		return
	}
	for ch := 0; ch < pr.numChannels; ch++ {
		pr.sincs[ch].Resample(src[ch], dst[ch])
	}
}

func (pr *PushResampler) OutputFrames() int {
	return pr.dstRate / 100
}
