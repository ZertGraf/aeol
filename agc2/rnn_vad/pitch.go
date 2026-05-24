package rnn_vad

import "sonora/fft"

const fftSize512 = 512

// kSubHarmonicMultipliers are used for sub-harmonic validation during extended pitch search.
// each pair (m[2k], m[2k+1]) validates a candidate at T/k against the initial pitch T.
var kSubHarmonicMultipliers = [14]float32{3, 2, 3, 2, 5, 2, 3, 2, 3, 2, 5, 2, 3, 2}

type pitchInfo struct {
	period   int
	strength float32
}

type pitchEstimator struct {
	lastPitch     pitchInfo
	yEnergy24kHz  [refineNumLags24kHz]float32
	pitchBuf12kHz [bufSize12kHz]float32
	autoCorr12kHz [numLags12kHz]float32
	fft512        fft.FFT
}

func newPitchEstimator() *pitchEstimator {
	return &pitchEstimator{
		fft512: fft.DefaultFactory(fftSize512),
	}
}

// estimate returns the pitch period at 48 kHz for the given pitch buffer.
// pitchBuffer must have length bufSize24kHz (864 samples at 24 kHz).
func (pe *pitchEstimator) estimate(pitchBuffer []float32) int {
	// decimate 2x: 24 kHz -> 12 kHz
	for i := 0; i < bufSize12kHz; i++ {
		pe.pitchBuf12kHz[i] = pitchBuffer[2*i]
	}

	pe.computeAutoCorr12kHz()

	best12, second12 := computePitchPeriod12kHz(pe.pitchBuf12kHz[:], pe.autoCorr12kHz[:])

	// scale candidates to 24 kHz
	best24 := best12 * 2
	second24 := second12 * 2

	// compute sliding frame square energies at 24 kHz
	pe.computeYEnergy24kHz(pitchBuffer)

	// refine at 24 kHz and scale to 48 kHz
	return pe.computeExtendedPitchPeriod48kHz(pitchBuffer, best24, second24)
}

// computeAutoCorr12kHz fills pe.autoCorr12kHz using FFT-based cross-correlation.
func (pe *pitchEstimator) computeAutoCorr12kHz() {
	buf := pe.pitchBuf12kHz[:]

	// reference: last frameSize20ms12kHz samples, time-reversed
	var hBuf [fftSize512]float32
	refStart := bufSize12kHz - frameSize20ms12kHz
	for i := 0; i < frameSize20ms12kHz; i++ {
		hBuf[i] = buf[refStart+frameSize20ms12kHz-1-i]
	}
	// zero pad to 512 (already zero from var declaration)

	// sliding chunk: first numLags12kHz + frameSize20ms12kHz samples
	chunkLen := numLags12kHz + frameSize20ms12kHz
	var xBuf [fftSize512]float32
	copy(xBuf[:chunkLen], buf[:chunkLen])

	pe.fft512.Forward(hBuf[:])
	pe.fft512.Forward(xBuf[:])

	// frequency-domain convolution (X * H, both real-packed)
	convolvePacked(xBuf[:], hBuf[:], fftSize512)

	pe.fft512.Inverse(xBuf[:])

	// extract auto-correlation: position frameSize20ms12kHz-1 onward
	start := frameSize20ms12kHz - 1
	for i := 0; i < numLags12kHz; i++ {
		pe.autoCorr12kHz[i] = xBuf[start+i]
	}
}

// convolvePacked multiplies packed real FFT spectra A and B in-place (A *= B).
// packed format: [re[0], re[N/2], re[1], im[1], re[2], im[2], ...]
func convolvePacked(a, b []float32, n int) {
	inv := 1.0 / float32(n)
	a[0] = a[0] * b[0] * inv
	a[1] = a[1] * b[1] * inv
	for k := 1; k < n/2; k++ {
		ar, ai := a[2*k], a[2*k+1]
		br, bi := b[2*k], b[2*k+1]
		a[2*k] = (ar*br - ai*bi) * inv
		a[2*k+1] = (ar*bi + ai*br) * inv
	}
}

// computePitchPeriod12kHz finds the best and second-best pitch candidates using
// a normalised cross-correlation criterion: autoCorr[lag]^2 / slidingEnergy.
// returns (bestLag, secondLag) in units of 12 kHz samples.
func computePitchPeriod12kHz(buf []float32, autoCorr []float32) (int, int) {
	// initial sliding energy over first frameSize20ms12kHz+1 samples
	energy := dotProduct(buf[:frameSize20ms12kHz+1], buf[:frameSize20ms12kHz+1])

	bestLag, secondLag := initialMinPitch12kHz, initialMinPitch12kHz
	var bestScore, secondScore float32

	for lag := 0; lag < numLags12kHz; lag++ {
		if energy < 1 {
			energy = 1
		}
		ac := autoCorr[lag]
		score := ac * ac / energy

		if score > bestScore {
			secondScore = bestScore
			secondLag = bestLag
			bestScore = score
			bestLag = initialMinPitch12kHz + lag
		} else if score > secondScore {
			secondScore = score
			secondLag = initialMinPitch12kHz + lag
		}

		// slide energy window: remove buf[lag]^2, add buf[lag+frameSize20ms12kHz+1]^2
		old := buf[lag]
		energy -= old * old
		next := buf[lag+frameSize20ms12kHz+1]
		energy += next * next
	}

	return bestLag, secondLag
}

// computeYEnergy24kHz fills pe.yEnergy24kHz with the squared energy of sliding
// frameSize20ms24kHz windows over pitchBuffer.
func (pe *pitchEstimator) computeYEnergy24kHz(pitchBuffer []float32) {
	yy := dotProduct(pitchBuffer[:frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz])

	for lag := 0; lag < refineNumLags24kHz; lag++ {
		old := pitchBuffer[lag]
		yy -= old * old
		next := pitchBuffer[lag+frameSize20ms24kHz]
		yy += next * next
		if yy < 1 {
			yy = 1
		}
		pe.yEnergy24kHz[lag] = yy
	}
}

// computePitchPeriod48kHz refines two 24 kHz candidates to 48 kHz by evaluating
// auto-correlation in ±2 neighbourhoods, then applying pseudo-interpolation.
func (pe *pitchEstimator) computePitchPeriod48kHz(pitchBuffer []float32, best24, second24 int) int {
	var bestLag24 int
	var bestScore float32

	for _, candidate := range [2]int{best24, second24} {
		lo := candidate - 2
		if lo < 0 {
			lo = 0
		}
		hi := candidate + 2
		if hi >= refineNumLags24kHz {
			hi = refineNumLags24kHz - 1
		}

		for lag := lo; lag <= hi; lag++ {
			ac := dotProduct(pitchBuffer[lag:lag+frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz])
			yy := pe.yEnergy24kHz[lag]
			if yy < 1 {
				yy = 1
			}
			score := ac * ac / yy

			if score > bestScore {
				bestScore = score
				bestLag24 = lag
			}
		}
	}

	// pseudo-interpolation
	var prev, curr, next float32
	curr = dotProduct(pitchBuffer[bestLag24:bestLag24+frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz])
	if bestLag24 > 0 {
		prev = dotProduct(pitchBuffer[bestLag24-1:bestLag24-1+frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz])
	}
	if bestLag24+1 < refineNumLags24kHz {
		next = dotProduct(pitchBuffer[bestLag24+1:bestLag24+1+frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz])
	}

	offset := getPitchPseudoInterpolationOffset(prev, curr, next)
	return 2*bestLag24 + offset
}

// computeExtendedPitchPeriod48kHz searches for sub-harmonics of the initial pitch
// and applies pitch tracking to return the final 48 kHz period and update lastPitch.
func (pe *pitchEstimator) computeExtendedPitchPeriod48kHz(pitchBuffer []float32, best24, second24 int) int {
	initialPitch48 := pe.computePitchPeriod48kHz(pitchBuffer, best24, second24)
	initialLag24 := initialPitch48 / 2

	// compute auto-correlation and energy at initial lag
	initialAC := dotProduct(
		pitchBuffer[initialLag24:initialLag24+frameSize20ms24kHz],
		pitchBuffer[:frameSize20ms24kHz],
	)
	initialEnergy := pe.yEnergy24kHz[initialLag24]
	if initialEnergy < 1 {
		initialEnergy = 1
	}

	bestPitch48 := initialPitch48
	bestStrength := initialAC * initialAC / initialEnergy

	maxDivisor := len(kSubHarmonicMultipliers) / 2
	for k := 0; k < maxDivisor; k++ {
		subLag24 := (initialLag24 + k) / (k + 2)
		if subLag24 < minPitch24kHz || subLag24 >= refineNumLags24kHz {
			continue
		}

		subAC := dotProduct(
			pitchBuffer[subLag24:subLag24+frameSize20ms24kHz],
			pitchBuffer[:frameSize20ms24kHz],
		)
		subEnergy := pe.yEnergy24kHz[subLag24]
		if subEnergy < 1 {
			subEnergy = 1
		}
		subScore := subAC * subAC / subEnergy

		// adaptive threshold based on sub-harmonic multipliers
		m0 := kSubHarmonicMultipliers[2*k]
		m1 := kSubHarmonicMultipliers[2*k+1]
		threshold := bestStrength * m1 / m0

		// lower threshold when close to last known pitch
		diff := subLag24*2 - pe.lastPitch.period
		if diff < 0 {
			diff = -diff
		}
		if diff <= 2 {
			threshold *= 0.85
		}

		if subScore > threshold {
			bestStrength = subScore
			bestPitch48 = subLag24 * 2

			// pseudo-interpolation for sub-harmonic candidate
			var prev, curr, next float32
			curr = subAC
			if subLag24 > 0 {
				prev = dotProduct(
					pitchBuffer[subLag24-1:subLag24-1+frameSize20ms24kHz],
					pitchBuffer[:frameSize20ms24kHz],
				)
			}
			if subLag24+1 < refineNumLags24kHz {
				next = dotProduct(
					pitchBuffer[subLag24+1:subLag24+1+frameSize20ms24kHz],
					pitchBuffer[:frameSize20ms24kHz],
				)
			}
			offset := getPitchPseudoInterpolationOffset(prev, curr, next)
			bestPitch48 = 2*subLag24 + offset
		}
	}

	// clamp to valid range
	if bestPitch48 < minPitch48kHz {
		bestPitch48 = minPitch48kHz
	}
	if bestPitch48 > maxPitch48kHz {
		bestPitch48 = maxPitch48kHz
	}

	// compute final strength
	finalLag24 := bestPitch48 / 2
	if finalLag24 >= refineNumLags24kHz {
		finalLag24 = refineNumLags24kHz - 1
	}
	finalAC := dotProduct(
		pitchBuffer[finalLag24:finalLag24+frameSize20ms24kHz],
		pitchBuffer[:frameSize20ms24kHz],
	)
	finalEnergy := pe.yEnergy24kHz[finalLag24]
	if finalEnergy < 1 {
		finalEnergy = 1
	}
	strength := finalAC / sqrtF32(finalEnergy*dotProduct(pitchBuffer[:frameSize20ms24kHz], pitchBuffer[:frameSize20ms24kHz]))

	pe.lastPitch = pitchInfo{period: bestPitch48, strength: strength}
	return bestPitch48
}

// dotProduct computes the inner product of two equal-length slices.
func dotProduct(a, b []float32) float32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float32
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// getPitchPseudoInterpolationOffset returns -1, 0, or +1 based on the shape of
// the auto-correlation around the peak (prev, curr, next).
func getPitchPseudoInterpolationOffset(prev, curr, next float32) int {
	if next > prev && next > curr {
		return 1
	}
	if prev > next && prev > curr {
		return -1
	}
	return 0
}

func sqrtF32(x float32) float32 {
	if x <= 0 {
		return 0
	}
	// Newton-Raphson single iteration starting from a float64 sqrt
	// is precise enough and avoids importing math for a simple helper.
	r := float32(1.0 / (1 << 15))
	// use a simple iterative approach via float32 multiplication
	// sufficient for normalisation purposes
	var y float32 = x
	// one Newton step: y = 0.5*(y + x/y)
	for i := 0; i < 8; i++ {
		y = 0.5 * (y + x/y)
	}
	_ = r
	return y
}
