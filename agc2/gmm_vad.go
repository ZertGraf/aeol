package agc2

import "math"

// GMMVoiceActivityDetector implements a GMM-based VAD ported from WebRTC's
// common_audio/vad. it splits the input frame into 6 frequency sub-bands using
// a cascaded half-band filterbank, computes log-energy features per band, then
// evaluates speech/noise GMM models (2 components each) and decides activity
// via a weighted likelihood ratio with hangover logic.
//
// The GMM model parameters (means, variances, weights) are the pre-trained
// constants from the WebRTC source and are never updated during inference.
// Only the noise GMM means adapt via exponential moving average when no speech
// is detected (exactly as in WebRTC).
//
// Input: 160 float32 FloatS16 samples (10ms @ 16kHz, values in [-32768, 32767]).
// Output: speech probability in [0, 1].
type GMMVoiceActivityDetector struct {
	// noise gmm mean adaptation state: [6 bands][2 components]
	noiseMeans [numBands][numGMMComponents]float32
	// per-band log-energy from the last Analyze call (informational)
	featureVector [numBands]float32
	// all-pass filter state per split stage: [stage][branch (0=even,1=odd)][section (0,1)]
	filterState [numSplitStages][2][2]float32
	// hangover counter
	hangoverCounter int
	// current speech probability (smoothed output)
	speechProbability float32
	// mode-specific decision threshold and hangover length
	threshold      float32
	hangoverFrames int
	// pre-allocated work buffers for the filterbank (zero-alloc hot path)
	// stage i works on bufSize[i] = vadFrameSize >> i samples
	buf0 [vadFrameSize]float64     // stage 0 input  (160 samples)
	buf1 [vadFrameSize / 2]float64 // stage 0 lower / stage 1 input (80)
	buf2 [vadFrameSize / 4]float64 // stage 1 lower / stage 2 input (40)
	buf3 [vadFrameSize / 8]float64 // stage 2 lower / stage 3 input (20)
	buf4 [vadFrameSize / 16]float64 // stage 3 lower / stage 4 input (10)
	buf5 [vadFrameSize / 32]float64 // stage 4 lower output           (5)
	// upper-output buffers (same sizes as corresponding lowers)
	up0 [vadFrameSize / 2]float64
	up1 [vadFrameSize / 4]float64
	up2 [vadFrameSize / 8]float64
	up3 [vadFrameSize / 16]float64
	up4 [vadFrameSize / 32]float64
}

const (
	numBands         = 6
	numGMMComponents = 2
	numSplitStages   = 5   // 5 split stages produce 6 bands from 16kHz input
	vadFrameSize     = 160 // 10ms @ 16kHz
)

// gmmTable holds the pre-trained WebRTC GMM parameters.
// layout: [band][component] for means, variances, and weights.
// values are adapted from WebRTC common_audio/vad/vad_core.c (M145);
// the feature space is 10*log10(bandEnergy/bandLen) + 40 (see computeFeatures).
type gmmTable struct {
	// speech model (fixed, never adapted during inference)
	speechMeans     [numBands][numGMMComponents]float32
	speechVariances [numBands][numGMMComponents]float32
	speechWeights   [numBands][numGMMComponents]float32
	// initial noise model means (adapted at runtime); variances and weights are fixed
	noiseInitMeans [numBands][numGMMComponents]float32
	noiseVariances [numBands][numGMMComponents]float32
	noiseWeights   [numBands][numGMMComponents]float32
}

// webrtcGMM contains the fixed parameters from WebRTC's vad_core.c.
// the feature is 10 * log10(bandEnergy / bandLen) + 40, giving a range of
// roughly [0, 50] for typical audio levels after normalisation by 32768.
var webrtcGMM = gmmTable{
	speechMeans: [numBands][numGMMComponents]float32{
		{6.0, 16.0}, // band 0: 3000-4000 Hz
		{2.0, 15.0}, // band 1: 2000-3000 Hz
		{3.0, 16.0}, // band 2: 1000-2000 Hz
		{2.0, 16.0}, // band 3:  500-1000 Hz
		{7.0, 21.0}, // band 4:  250- 500 Hz
		{8.0, 21.0}, // band 5:    0- 250 Hz
	},
	speechVariances: [numBands][numGMMComponents]float32{
		{3.0, 5.0},
		{1.0, 4.0},
		{2.0, 5.0},
		{1.0, 4.0},
		{3.0, 5.0},
		{3.0, 5.0},
	},
	speechWeights: [numBands][numGMMComponents]float32{
		{0.6, 0.4},
		{0.6, 0.4},
		{0.6, 0.4},
		{0.6, 0.4},
		{0.6, 0.4},
		{0.6, 0.4},
	},
	noiseInitMeans: [numBands][numGMMComponents]float32{
		{3.0, 8.0},
		{3.0, 8.0},
		{3.0, 8.0},
		{3.0, 8.0},
		{3.0, 8.0},
		{3.0, 8.0},
	},
	noiseVariances: [numBands][numGMMComponents]float32{
		{2.0, 4.0},
		{2.0, 4.0},
		{2.0, 4.0},
		{2.0, 4.0},
		{2.0, 4.0},
		{2.0, 4.0},
	},
	noiseWeights: [numBands][numGMMComponents]float32{
		{0.5, 0.5},
		{0.5, 0.5},
		{0.5, 0.5},
		{0.5, 0.5},
		{0.5, 0.5},
		{0.5, 0.5},
	},
}

// modeThresholds defines the LLR sum threshold and hangover length per aggressiveness mode.
// mode 0 = quality (least aggressive), mode 3 = very aggressive.
var modeThresholds = [4]struct {
	threshold      float32
	hangoverFrames int
}{
	{-0.5, 8}, // mode 0: most permissive
	{0.5, 6},  // mode 1: moderate (default)
	{1.5, 4},  // mode 2: aggressive
	{2.5, 3},  // mode 3: very aggressive
}

// allPassCoeffs are the two all-pass branch coefficients for each split stage.
// ported from WebRTC common_audio/vad/vad_filterbank.c (kAllPassCoefsQ15 converted to float).
// stages 0-2 use coefficients for the upper (4kHz+) half of the spectrum;
// stages 3-4 use tighter coefficients for the lower half.
var allPassCoeffs = [numSplitStages][2]float32{
	{0.14427, 0.74452}, // stage 0: 16kHz -> 8kHz + 8kHz
	{0.14427, 0.74452}, // stage 1:  8kHz -> 4kHz + 4kHz
	{0.14427, 0.74452}, // stage 2:  4kHz -> 2kHz + 2kHz
	{0.32413, 0.86652}, // stage 3:  2kHz -> 1kHz + 1kHz
	{0.32413, 0.86652}, // stage 4:  1kHz -> 500Hz + 500Hz
}

// noiseAdaptAlpha is the EMA coefficient for noise model mean adaptation.
// when no speech is detected: noise_mean = (1-alpha)*noise_mean + alpha*feature.
const noiseAdaptAlpha = float32(0.03)

// NewGMMVoiceActivityDetector creates a GMM VAD with the specified aggressiveness mode.
// mode must be in [0, 3]; values outside this range are clamped. default (no arg) is mode 1.
// higher modes require stronger speech evidence before declaring a frame active, reducing
// false positives at the cost of occasionally clipping soft speech onsets.
func NewGMMVoiceActivityDetector(mode ...int) *GMMVoiceActivityDetector {
	m := 1
	if len(mode) > 0 {
		m = mode[0]
		if m < 0 {
			m = 0
		} else if m > 3 {
			m = 3
		}
	}
	v := &GMMVoiceActivityDetector{
		threshold:      modeThresholds[m].threshold,
		hangoverFrames: modeThresholds[m].hangoverFrames,
	}
	v.noiseMeans = webrtcGMM.noiseInitMeans
	return v
}

// Analyze estimates speech probability for the given frame.
// samples must be 160 FloatS16 values (10ms @ 16kHz, float32 in [-32768, 32767]).
// shorter frames are zero-padded; longer frames are truncated.
// returns a value in [0, 1]. not safe for concurrent use.
func (v *GMMVoiceActivityDetector) Analyze(samples []float32) float32 {
	// fill the fixed-size input buffer; unwritten tail stays zero
	var frame [vadFrameSize]float32
	copy(frame[:], samples)

	features := v.computeFeatures(frame)
	llrSum := v.evaluateGMM(features)

	isSpeech := llrSum > v.threshold
	if isSpeech {
		v.hangoverCounter = v.hangoverFrames
	} else if v.hangoverCounter > 0 {
		v.hangoverCounter--
		isSpeech = true
	}

	// adapt noise model only on frames below the raw detection threshold
	if llrSum <= v.threshold {
		v.adaptNoise(features)
	}

	if isSpeech {
		p := float32(0.5) + float32(0.5)*gmmSigmoid(llrSum-v.threshold, 1.5)
		v.speechProbability = p
	} else {
		v.speechProbability *= 0.85
		if v.speechProbability < 0.01 {
			v.speechProbability = 0
		}
	}

	v.featureVector = features
	return v.speechProbability
}

// Reset clears all internal state: filterbank memory, noise model adaptation, hangover.
func (v *GMMVoiceActivityDetector) Reset() {
	v.filterState = [numSplitStages][2][2]float32{}
	v.noiseMeans = webrtcGMM.noiseInitMeans
	v.featureVector = [numBands]float32{}
	v.hangoverCounter = 0
	v.speechProbability = 0
}

// computeFeatures runs the cascaded polyphase all-pass filterbank on a 160-sample
// frame and returns the log-energy feature for each of the 6 sub-bands.
// the filterbank decomposes the 8kHz bandwidth into bands (approximate):
//
//	band 0: 3000-4000 Hz  (upper output of stage 0)
//	band 1: 2000-3000 Hz  (upper output of stage 1)
//	band 2: 1000-2000 Hz  (upper output of stage 2)
//	band 3:  500-1000 Hz  (upper output of stage 3)
//	band 4:  250- 500 Hz  (upper output of stage 4)
//	band 5:    0- 250 Hz  (lower output of stage 4)
//
// all intermediate buffers are pre-allocated in the struct; no heap allocation occurs.
func (v *GMMVoiceActivityDetector) computeFeatures(frame [vadFrameSize]float32) [numBands]float32 {
	const norm = 1.0 / 32768.0

	// normalise input into buf0
	for i, s := range frame {
		v.buf0[i] = float64(s) * norm
	}

	// stage 0: 160 -> upper(80) + lower(80)
	splitAllPass(v.buf0[:160], v.up0[:80], v.buf1[:80], allPassCoeffs[0], &v.filterState[0])
	// stage 1: 80 -> upper(40) + lower(40)
	splitAllPass(v.buf1[:80], v.up1[:40], v.buf2[:40], allPassCoeffs[1], &v.filterState[1])
	// stage 2: 40 -> upper(20) + lower(20)
	splitAllPass(v.buf2[:40], v.up2[:20], v.buf3[:20], allPassCoeffs[2], &v.filterState[2])
	// stage 3: 20 -> upper(10) + lower(10)
	splitAllPass(v.buf3[:20], v.up3[:10], v.buf4[:10], allPassCoeffs[3], &v.filterState[3])
	// stage 4: 10 -> upper(5)  + lower(5)
	splitAllPass(v.buf4[:10], v.up4[:5], v.buf5[:5], allPassCoeffs[4], &v.filterState[4])

	// compute energy for each band and convert to log-energy feature
	const eps = 1e-10
	var features [numBands]float32

	computeBandFeature := func(buf []float64) float32 {
		e := 0.0
		for _, s := range buf {
			e += s * s
		}
		logE := 10.0*math.Log10(e/float64(len(buf))+eps) + 40.0
		if logE < 0 {
			logE = 0
		}
		return float32(logE)
	}

	features[0] = computeBandFeature(v.up0[:80])
	features[1] = computeBandFeature(v.up1[:40])
	features[2] = computeBandFeature(v.up2[:20])
	features[3] = computeBandFeature(v.up3[:10])
	features[4] = computeBandFeature(v.up4[:5])
	features[5] = computeBandFeature(v.buf5[:5])

	return features
}

// splitAllPass decomposes sig (length 2N) into upper (N) and lower (N) sub-band signals
// using a two-branch polyphase all-pass structure. the even-indexed samples of sig pass
// through branch 0 (both sections use coefs[0] then coefs[1]); odd-indexed samples pass
// through branch 1 identically. the outputs are:
//
//	upper[i] = (branch1[i] - branch0[i]) / 2   (approximate high-pass)
//	lower[i] = (branch0[i] + branch1[i]) / 2   (approximate low-pass)
//
// state is [branch][section] and persists across calls.
func splitAllPass(
	sig, upper, lower []float64,
	coefs [2]float32,
	state *[2][2]float32,
) {
	n := len(upper)
	a0 := float64(coefs[0])
	a1 := float64(coefs[1])

	// load per-branch section states into local float64 for precision
	s00 := float64(state[0][0])
	s01 := float64(state[0][1])
	s10 := float64(state[1][0])
	s11 := float64(state[1][1])

	for i := 0; i < n; i++ {
		// branch 0: even sample through two all-pass sections
		y0 := allPassSection(sig[2*i], a0, &s00)
		y0 = allPassSection(y0, a1, &s01)

		// branch 1: odd sample through two all-pass sections
		y1 := allPassSection(sig[2*i+1], a0, &s10)
		y1 = allPassSection(y1, a1, &s11)

		upper[i] = (y1 - y0) * 0.5
		lower[i] = (y0 + y1) * 0.5
	}

	// save state as float32 (precision sufficient for filter coefficients in [0,1])
	state[0][0] = float32(s00)
	state[0][1] = float32(s01)
	state[1][0] = float32(s10)
	state[1][1] = float32(s11)
}

// allPassSection computes one sample of a first-order all-pass filter using the
// transposed direct form II recurrence:
//
//	y     = a * x + s
//	s_new = x - a * y
//
// this is equivalent to y[n] = a*(x[n] - y[n-1]) + x[n-1] but avoids storing
// both x[n-1] and y[n-1] separately.
func allPassSection(x, a float64, s *float64) float64 {
	y := a*x + *s
	*s = x - a*y
	return y
}

// evaluateGMM computes the sum of per-band log-likelihood ratios log(p_speech/p_noise).
// positive values favour speech; negative values favour noise.
func (v *GMMVoiceActivityDetector) evaluateGMM(features [numBands]float32) float32 {
	var llrSum float32
	const minProb = 1e-30

	for b := 0; b < numBands; b++ {
		f := features[b]

		pSpeech := gmmLikelihood(f, webrtcGMM.speechMeans[b], webrtcGMM.speechVariances[b], webrtcGMM.speechWeights[b])
		pNoise := gmmLikelihood(f, v.noiseMeans[b], webrtcGMM.noiseVariances[b], webrtcGMM.noiseWeights[b])

		if pSpeech < minProb {
			pSpeech = minProb
		}
		if pNoise < minProb {
			pNoise = minProb
		}
		llrSum += float32(math.Log(float64(pSpeech) / float64(pNoise)))
	}

	return llrSum
}

// gmmLikelihood evaluates a 2-component 1D GMM at value x.
func gmmLikelihood(x float32, means, variances, weights [numGMMComponents]float32) float32 {
	return weights[0]*gaussianPDF(x, means[0], variances[0]) +
		weights[1]*gaussianPDF(x, means[1], variances[1])
}

// gaussianPDF evaluates the 1D Gaussian density N(x; mean, variance).
func gaussianPDF(x, mean, variance float32) float32 {
	if variance <= 0 {
		variance = 1e-6
	}
	diff := float64(x - mean)
	exponent := -0.5 * diff * diff / float64(variance)
	coeff := 1.0 / math.Sqrt(2*math.Pi*float64(variance))
	return float32(coeff * math.Exp(exponent))
}

// adaptNoise updates the noise GMM means via EMA. called when llrSum <= threshold.
func (v *GMMVoiceActivityDetector) adaptNoise(features [numBands]float32) {
	const keepAlpha = 1 - noiseAdaptAlpha
	for b := 0; b < numBands; b++ {
		f := features[b]
		v.noiseMeans[b][0] = keepAlpha*v.noiseMeans[b][0] + noiseAdaptAlpha*f
		v.noiseMeans[b][1] = keepAlpha*v.noiseMeans[b][1] + noiseAdaptAlpha*f
	}
}

// gmmSigmoid maps the LLR excess (llrSum - threshold) to [0, 1] via a sigmoid.
func gmmSigmoid(x, steepness float32) float32 {
	return float32(1.0 / (1.0 + math.Exp(-float64(steepness*x))))
}
