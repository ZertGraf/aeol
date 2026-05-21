package dsp

import (
	"math"

	"sonora/simd"
)

const (
	sincKernelSize       = 32
	sincResamplerOffsets = 32
	sincBufferSize       = sincKernelSize * (sincResamplerOffsets + 1)
)

type SincResampler struct {
	ioRatio     float64
	virtualPos  float64
	blockSize   int
	inputBuffer []float32
	kernelTable [sincBufferSize]float64
	r1, r2      int
	backend     simd.Backend
}

func NewSincResampler(ioRatio float64, requestFrames int) *SincResampler {
	s := &SincResampler{
		ioRatio:   ioRatio,
		blockSize: requestFrames,
		backend:   simd.Default(),
	}
	s.inputBuffer = make([]float32, sincKernelSize+requestFrames)
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
	srcLen := len(src)
	dstLen := len(dst)
	srcIdx := 0
	dstIdx := 0

	for dstIdx < dstLen {
		intPos := int(s.virtualPos)
		frac := s.virtualPos - float64(intPos)

		for intPos+sincKernelSize > srcIdx+len(s.inputBuffer) && srcIdx < srcLen {
			srcIdx++
		}

		offsetIdx := int(frac * float64(sincResamplerOffsets))
		if offsetIdx >= sincResamplerOffsets {
			offsetIdx = sincResamplerOffsets - 1
		}

		k1Start := offsetIdx * sincKernelSize
		k2Start := (offsetIdx + 1) * sincKernelSize
		interpFactor := frac*float64(sincResamplerOffsets) - float64(offsetIdx)

		inputStart := intPos
		if inputStart < 0 {
			inputStart = 0
		}
		inputEnd := inputStart + sincKernelSize
		if inputEnd > srcLen {
			break
		}

		input := src[inputStart:inputEnd]
		k1 := s.kernelTable[k1Start : k1Start+sincKernelSize]
		k2 := s.kernelTable[k2Start : k2Start+sincKernelSize]

		dst[dstIdx] = s.backend.ConvolveSinc(input, k1, k2, interpFactor)
		dstIdx++

		s.virtualPos += s.ioRatio
	}

	s.virtualPos -= float64(srcLen)
	return dstIdx
}

func (s *SincResampler) Reset() {
	s.virtualPos = 0
	clear(s.inputBuffer)
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
	sinc        *SincResampler
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
		pr.sinc = NewSincResampler(ioRatio, dstFrames)
	}

	for ch := 0; ch < numChannels; ch++ {
		pr.srcBuf[ch] = make([]float32, srcFrames)
		pr.dstBuf[ch] = make([]float32, dstFrames)
	}

	return pr
}

func (pr *PushResampler) Resample(src [][]float32, dst [][]float32) {
	if pr.sinc == nil {
		for ch := 0; ch < pr.numChannels; ch++ {
			copy(dst[ch], src[ch])
		}
		return
	}
	for ch := 0; ch < pr.numChannels; ch++ {
		pr.sinc.Reset()
		pr.sinc.Resample(src[ch], dst[ch])
	}
}

func (pr *PushResampler) OutputFrames() int {
	return pr.dstRate / 100
}
