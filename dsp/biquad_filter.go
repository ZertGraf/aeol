package dsp

type BiQuadCoefficients struct {
	B [3]float32
	A [2]float32
}

type BiQuadFilter struct {
	coeffs BiQuadCoefficients
	x1, x2 float32
	y1, y2 float32
}

func NewBiQuadFilter(coeffs BiQuadCoefficients) *BiQuadFilter {
	return &BiQuadFilter{coeffs: coeffs}
}

func (f *BiQuadFilter) Process(in []float32, out []float32) {
	b0 := f.coeffs.B[0]
	b1 := f.coeffs.B[1]
	b2 := f.coeffs.B[2]
	a1 := f.coeffs.A[0]
	a2 := f.coeffs.A[1]

	x1 := f.x1
	x2 := f.x2
	y1 := f.y1
	y2 := f.y2

	n := min(len(in), len(out))
	for i := 0; i < n; i++ {
		x0 := in[i]
		y0 := b0*x0 + b1*x1 + b2*x2 - a1*y1 - a2*y2
		out[i] = y0
		x2 = x1
		x1 = x0
		y2 = y1
		y1 = y0
	}

	f.x1 = x1
	f.x2 = x2
	f.y1 = y1
	f.y2 = y2
}

func (f *BiQuadFilter) ProcessInPlace(data []float32) {
	f.Process(data, data)
}

func (f *BiQuadFilter) Reset() {
	f.x1 = 0
	f.x2 = 0
	f.y1 = 0
	f.y2 = 0
}

type CascadedBiQuadFilter struct {
	stages []*BiQuadFilter
}

func NewCascadedBiQuadFilter(coeffs []BiQuadCoefficients) *CascadedBiQuadFilter {
	stages := make([]*BiQuadFilter, len(coeffs))
	for i, c := range coeffs {
		stages[i] = NewBiQuadFilter(c)
	}
	return &CascadedBiQuadFilter{stages: stages}
}

func (c *CascadedBiQuadFilter) Process(in []float32, out []float32) {
	if len(c.stages) == 0 {
		copy(out, in)
		return
	}
	c.stages[0].Process(in, out)
	for i := 1; i < len(c.stages); i++ {
		c.stages[i].ProcessInPlace(out)
	}
}

func (c *CascadedBiQuadFilter) ProcessInPlace(data []float32) {
	for _, s := range c.stages {
		s.ProcessInPlace(data)
	}
}

func (c *CascadedBiQuadFilter) Reset() {
	for _, s := range c.stages {
		s.Reset()
	}
}
