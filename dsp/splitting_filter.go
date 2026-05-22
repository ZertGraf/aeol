package dsp

const (
	SamplesPerBand               = 160
	TwoBandFilterSamplesPerFrame = 320
	qmfStateSize                 = 6
)

var allPassFilter1 = [3]float32{0.09793091, 0.56430054, 0.8737335}
var allPassFilter2 = [3]float32{0.32551575, 0.7486267, 0.9614563}

func allpassQMF(inData []float32, outData []float32, coefficients []float32, state []float32) {
	dataLength := SamplesPerBand

	diff := inData[0] - state[1]
	outData[0] = state[0] + coefficients[0]*diff
	for k := 1; k < dataLength; k++ {
		diff = inData[k] - outData[k-1]
		outData[k] = inData[k-1] + coefficients[0]*diff
	}
	state[0] = inData[dataLength-1]
	state[1] = outData[dataLength-1]

	diff = outData[0] - state[3]
	inData[0] = state[2] + coefficients[1]*diff
	for k := 1; k < dataLength; k++ {
		diff = outData[k] - inData[k-1]
		inData[k] = outData[k-1] + coefficients[1]*diff
	}
	state[2] = outData[dataLength-1]
	state[3] = inData[dataLength-1]

	diff = inData[0] - state[5]
	outData[0] = state[4] + coefficients[2]*diff
	for k := 1; k < dataLength; k++ {
		diff = inData[k] - outData[k-1]
		outData[k] = inData[k-1] + coefficients[2]*diff
	}
	state[4] = inData[dataLength-1]
	state[5] = outData[dataLength-1]
}

func analysisQMF(inData []float32, lowBand []float32, highBand []float32, filterState1 []float32, filterState2 []float32) {
	var halfIn1 [SamplesPerBand]float32
	var halfIn2 [SamplesPerBand]float32
	for i := 0; i < SamplesPerBand; i++ {
		halfIn2[i] = inData[2*i]
		halfIn1[i] = inData[2*i+1]
	}

	var filter1 [SamplesPerBand]float32
	var filter2 [SamplesPerBand]float32
	allpassQMF(halfIn1[:], filter1[:], allPassFilter1[:], filterState1)
	allpassQMF(halfIn2[:], filter2[:], allPassFilter2[:], filterState2)

	for i := 0; i < SamplesPerBand; i++ {
		lowBand[i] = (filter1[i] + filter2[i]) * 0.5
		highBand[i] = (filter1[i] - filter2[i]) * 0.5
	}
}

func synthesisQMF(lowBand []float32, highBand []float32, outData []float32, filterState1 []float32, filterState2 []float32) {
	var halfIn1 [SamplesPerBand]float32
	var halfIn2 [SamplesPerBand]float32
	for i := 0; i < SamplesPerBand; i++ {
		halfIn1[i] = lowBand[i] + highBand[i]
		halfIn2[i] = lowBand[i] - highBand[i]
	}

	var filter1 [SamplesPerBand]float32
	var filter2 [SamplesPerBand]float32
	allpassQMF(halfIn1[:], filter1[:], allPassFilter2[:], filterState1)
	allpassQMF(halfIn2[:], filter2[:], allPassFilter1[:], filterState2)

	for i := 0; i < SamplesPerBand; i++ {
		outData[2*i] = filter2[i]
		outData[2*i+1] = filter1[i]
	}
}

type TwoBandsStates struct {
	analysisState1  [qmfStateSize]float32
	analysisState2  [qmfStateSize]float32
	synthesisState1 [qmfStateSize]float32
	synthesisState2 [qmfStateSize]float32
}

type SplittingFilter struct {
	numBands   int
	twoBands   []TwoBandsStates
	threeBands []*ThreeBandFilterBank
}

func NewSplittingFilter(numChannels int, numBands int) *SplittingFilter {
	sf := &SplittingFilter{numBands: numBands}
	if numBands == 2 {
		sf.twoBands = make([]TwoBandsStates, numChannels)
	} else if numBands == 3 {
		sf.threeBands = make([]*ThreeBandFilterBank, numChannels)
		for i := 0; i < numChannels; i++ {
			sf.threeBands[i] = NewThreeBandFilterBank()
		}
	}
	return sf
}

func (sf *SplittingFilter) Analysis(data [][]float32, bands [][][]float32) {
	numChannels := len(data)
	if sf.numBands == 2 {
		for i := 0; i < numChannels; i++ {
			state := &sf.twoBands[i]
			analysisQMF(data[i], bands[i][0], bands[i][1], state.analysisState1[:], state.analysisState2[:])
		}
	} else if sf.numBands == 3 {
		for i := 0; i < numChannels; i++ {
			sf.threeBands[i].Analysis(data[i], bands[i])
		}
	}
}

func (sf *SplittingFilter) Synthesis(bands [][][]float32, data [][]float32) {
	numChannels := len(data)
	if sf.numBands == 2 {
		for i := 0; i < numChannels; i++ {
			state := &sf.twoBands[i]
			synthesisQMF(bands[i][0], bands[i][1], data[i], state.synthesisState1[:], state.synthesisState2[:])
		}
	} else if sf.numBands == 3 {
		for i := 0; i < numChannels; i++ {
			sf.threeBands[i].Synthesis(bands[i], data[i])
		}
	}
}
