package rnn_vad

// ActivationFunction represents the activation function for a neural network cell.
type ActivationFunction int

const (
	TansigApproximated ActivationFunction = iota
	SigmoidApproximated
)

// tansigTable is an auto-generated table from rnnoise used for approximating tansig/tanh.
var tansigTable = [201]float32{
	0.000000, 0.039979, 0.079830, 0.119427, 0.158649,
	0.197375, 0.235496, 0.272905, 0.309507, 0.345214,
	0.379949, 0.413644, 0.446244, 0.477700, 0.507977,
	0.537050, 0.564900, 0.591519, 0.616909, 0.641077,
	0.664037, 0.685809, 0.706419, 0.725897, 0.744277,
	0.761594, 0.777888, 0.793199, 0.807569, 0.821040,
	0.833655, 0.845456, 0.856485, 0.866784, 0.876393,
	0.885352, 0.893698, 0.901468, 0.908698, 0.915420,
	0.921669, 0.927473, 0.932862, 0.937863, 0.942503,
	0.946806, 0.950795, 0.954492, 0.957917, 0.961090,
	0.964028, 0.966747, 0.969265, 0.971594, 0.973749,
	0.975743, 0.977587, 0.979293, 0.980869, 0.982327,
	0.983675, 0.984921, 0.986072, 0.987136, 0.988119,
	0.989027, 0.989867, 0.990642, 0.991359, 0.992020,
	0.992631, 0.993196, 0.993718, 0.994199, 0.994644,
	0.995055, 0.995434, 0.995784, 0.996108, 0.996407,
	0.996682, 0.996937, 0.997172, 0.997389, 0.997590,
	0.997775, 0.997946, 0.998104, 0.998249, 0.998384,
	0.998508, 0.998623, 0.998728, 0.998826, 0.998916,
	0.999000, 0.999076, 0.999147, 0.999213, 0.999273,
	0.999329, 0.999381, 0.999428, 0.999472, 0.999513,
	0.999550, 0.999585, 0.999617, 0.999646, 0.999673,
	0.999699, 0.999722, 0.999743, 0.999763, 0.999781,
	0.999798, 0.999813, 0.999828, 0.999841, 0.999853,
	0.999865, 0.999875, 0.999885, 0.999893, 0.999902,
	0.999909, 0.999916, 0.999923, 0.999929, 0.999934,
	0.999939, 0.999944, 0.999948, 0.999952, 0.999956,
	0.999959, 0.999962, 0.999965, 0.999968, 0.999970,
	0.999973, 0.999975, 0.999977, 0.999978, 0.999980,
	0.999982, 0.999983, 0.999984, 0.999986, 0.999987,
	0.999988, 0.999989, 0.999990, 0.999990, 0.999991,
	0.999992, 0.999992, 0.999993, 0.999994, 0.999994,
	0.999994, 0.999995, 0.999995, 0.999996, 0.999996,
	0.999996, 0.999997, 0.999997, 0.999997, 0.999997,
	0.999997, 0.999998, 0.999998, 0.999998, 0.999998,
	0.999998, 0.999998, 0.999999, 0.999999, 0.999999,
	0.999999, 0.999999, 0.999999, 0.999999, 0.999999,
	0.999999, 0.999999, 0.999999, 0.999999, 0.999999,
	1.000000, 1.000000, 1.000000, 1.000000, 1.000000,
	1.000000, 1.000000, 1.000000, 1.000000, 1.000000,
	1.000000,
}

func tansigApproximated(x float32) float32 {
	if x >= 8 {
		return 1.0
	}
	if x <= -8 {
		return -1.0
	}
	sign := float32(1.0)
	if x < 0 {
		x = -x
		sign = -1.0
	}
	i := int(0.5 + 25.0*x)
	x -= 0.04 * float32(i)
	y := tansigTable[i]
	dy := 1.0 - y*y
	y = y + x*dy*(1.0 - y*x)
	return sign * y
}

func sigmoidApproximated(x float32) float32 {
	return 0.5 + 0.5*tansigApproximated(0.5*x)
}

func getActivationFunction(act ActivationFunction) func(float32) float32 {
	switch act {
	case TansigApproximated:
		return tansigApproximated
	case SigmoidApproximated:
		return sigmoidApproximated
	}
	return nil
}

// FullyConnectedLayer represents a fully-connected layer.
type FullyConnectedLayer struct {
	inputSize          int
	outputSize         int
	bias               []float32
	weights            []float32
	activationFunction func(float32) float32
	output             []float32
}

// NewFullyConnectedLayer creates a new fully connected layer.
func NewFullyConnectedLayer(inputSize, outputSize int, bias, weights []float32, act ActivationFunction) *FullyConnectedLayer {
	return &FullyConnectedLayer{
		inputSize:          inputSize,
		outputSize:         outputSize,
		bias:               bias,
		weights:            weights,
		activationFunction: getActivationFunction(act),
		output:             make([]float32, outputSize),
	}
}

// ComputeOutput computes the fully-connected layer output.
func (fc *FullyConnectedLayer) ComputeOutput(input []float32) {
	for o := 0; o < fc.outputSize; o++ {
		sum := fc.bias[o]
		weightOffset := o * fc.inputSize
		for i := 0; i < fc.inputSize; i++ {
			sum += input[i] * fc.weights[weightOffset+i]
		}
		fc.output[o] = fc.activationFunction(sum)
	}
}

// GatedRecurrentLayer represents a recurrent layer with gated recurrent units (GRUs).
type GatedRecurrentLayer struct {
	inputSize        int
	outputSize       int
	bias             []float32
	weights          []float32
	recurrentWeights []float32
	state            []float32

	// Pre-allocated buffers to prevent allocation during ComputeOutput
	update      []float32
	reset       []float32
	resetXState []float32
}

// NewGatedRecurrentLayer creates a new GRU layer.
func NewGatedRecurrentLayer(inputSize, outputSize int, bias, weights, recurrentWeights []float32) *GatedRecurrentLayer {
	return &GatedRecurrentLayer{
		inputSize:        inputSize,
		outputSize:       outputSize,
		bias:             bias,
		weights:          weights,
		recurrentWeights: recurrentWeights,
		state:            make([]float32, outputSize),
		update:           make([]float32, outputSize),
		reset:            make([]float32, outputSize),
		resetXState:      make([]float32, outputSize),
	}
}

// Reset clears the GRU state.
func (g *GatedRecurrentLayer) Reset() {
	for i := range g.state {
		g.state[i] = 0.0
	}
}

// ComputeOutput computes the recurrent layer output and updates the state.
func (g *GatedRecurrentLayer) ComputeOutput(input []float32) {
	strideWeights := g.inputSize * g.outputSize
	strideRecurrentWeights := g.outputSize * g.outputSize

	// Update gate (g=0)
	computeUpdateResetGate(
		g.inputSize, g.outputSize, input, g.state,
		g.bias[0:g.outputSize],
		g.weights[0:strideWeights],
		g.recurrentWeights[0:strideRecurrentWeights],
		g.update,
	)

	// Reset gate (g=1)
	computeUpdateResetGate(
		g.inputSize, g.outputSize, input, g.state,
		g.bias[g.outputSize:2*g.outputSize],
		g.weights[strideWeights:2*strideWeights],
		g.recurrentWeights[strideRecurrentWeights:2*strideRecurrentWeights],
		g.reset,
	)

	// State gate (g=2)
	computeStateGate(
		g.inputSize, g.outputSize, input, g.update, g.reset,
		g.bias[2*g.outputSize:3*g.outputSize],
		g.weights[2*strideWeights:3*strideWeights],
		g.recurrentWeights[2*strideRecurrentWeights:3*strideRecurrentWeights],
		g.state,
		g.resetXState,
	)
}

func computeUpdateResetGate(inputSize, outputSize int, input, state, bias, weights, recurrentWeights, gate []float32) {
	for o := 0; o < outputSize; o++ {
		x := bias[o]
		wOffset := o * inputSize
		for i := 0; i < inputSize; i++ {
			x += input[i] * weights[wOffset+i]
		}
		rOffset := o * outputSize
		for i := 0; i < outputSize; i++ {
			x += state[i] * recurrentWeights[rOffset+i]
		}
		gate[o] = sigmoidApproximated(x)
	}
}

func computeStateGate(inputSize, outputSize int, input, update, reset, bias, weights, recurrentWeights, state, resetXState []float32) {
	for o := 0; o < outputSize; o++ {
		resetXState[o] = state[o] * reset[o]
	}

	for o := 0; o < outputSize; o++ {
		x := bias[o]
		wOffset := o * inputSize
		for i := 0; i < inputSize; i++ {
			x += input[i] * weights[wOffset+i]
		}
		rOffset := o * outputSize
		for i := 0; i < outputSize; i++ {
			x += resetXState[i] * recurrentWeights[rOffset+i]
		}

		reluX := x
		if reluX < 0 {
			reluX = 0
		}

		state[o] = update[o]*state[o] + (1.0-update[o])*reluX
	}
}

// FeatureVectorSize is the size of the input feature vector.
const FeatureVectorSize = 42

// RNNVad is a recurrent network with hard-coded architecture and weights for VAD.
type RNNVad struct {
	input  *FullyConnectedLayer
	hidden *GatedRecurrentLayer
	output *FullyConnectedLayer
}

// NewRNNVad creates a new RNNVad instance.
func NewRNNVad() *RNNVad {
	return &RNNVad{
		input: NewFullyConnectedLayer(
			42, 24,
			inputDenseBias[:],
			inputDenseWeights[:],
			TansigApproximated,
		),
		hidden: NewGatedRecurrentLayer(
			24, 24,
			hiddenGruBias[:],
			hiddenGruWeights[:],
			hiddenGruRecurrentWeights[:],
		),
		output: NewFullyConnectedLayer(
			24, 1,
			outputDenseBias[:],
			outputDenseWeights[:],
			SigmoidApproximated,
		),
	}
}

// Reset resets the state of the RNN.
func (r *RNNVad) Reset() {
	r.hidden.Reset()
}

// ComputeVadProbability computes the VAD probability for the given feature vector.
func (r *RNNVad) ComputeVadProbability(featureVector []float32, isSilence bool) float32 {
	if isSilence {
		r.Reset()
		return 0.0
	}
	r.input.ComputeOutput(featureVector)
	r.hidden.ComputeOutput(r.input.output)
	r.output.ComputeOutput(r.hidden.state)
	return r.output.output[0]
}
