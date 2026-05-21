package aec3

type SuppressionFilter struct {
	config SuppressorConfig
}

func NewSuppressionFilter(config SuppressorConfig) *SuppressionFilter {
	return &SuppressionFilter{
		config: config,
	}
}

// Process applies spectral suppression to the linear output.
func (sf *SuppressionFilter) Process(captureFft *FftData, linearOutput *FftData, erle float32) {
	var capturePower [FFTSizeBy2Plus1]float32
	var errorPower [FFTSizeBy2Plus1]float32

	for k := 0; k < FFTSizeBy2Plus1; k++ {
		capturePower[k] = captureFft.Re[k]*captureFft.Re[k] + captureFft.Im[k]*captureFft.Im[k]
		errorPower[k] = linearOutput.Re[k]*linearOutput.Re[k] + linearOutput.Im[k]*linearOutput.Im[k]
	}

	for k := 0; k < FFTSizeBy2Plus1; k++ {
		// Basic spectral suppression gain calculation based on residual echo power vs capture power
		
		// The estimated echo power is capture power minus error power (as error = capture - echo).
		// In reality, if errorPower is much smaller than capturePower, echo power was high.
		// If echo was removed perfectly, linearOutput only has near-end signal.
		// Here we compute a simple Wiener filter-like gain based on ERLE.
		
		var gain float32 = 1.0
		
		if capturePower[k] > 1e-10 {
			// A simple heuristic: residual echo power is estimated echo power / ERLE.
			// estimated echo power is roughly capturePower - errorPower, bounded by 0.
			estEchoPower := capturePower[k] - errorPower[k]
			if estEchoPower < 0 {
				estEchoPower = 0
			}
			
			// residual echo power
			residualEchoPower := estEchoPower / erle
			
			// Suppress based on signal-to-residual-echo ratio
			if residualEchoPower > 1e-10 {
				snr := errorPower[k] / residualEchoPower
				// standard Wiener-like suppression: gain = SNR / (SNR + 1)
				gain = snr / (snr + 1.0)
			}
		}

		// Apply lower bounds based on mask config to avoid over-suppression artifacts
		// For simplicity, just use EnrTransparent as a baseline floor
		floor := sf.config.NormalTuning.MaskLf.EnrTransparent
		if k > 32 { // High frequency
			floor = sf.config.NormalTuning.MaskHf.EnrTransparent
		}
		
		if gain < floor {
			gain = floor
		}
		
		// Apply suppression gain
		linearOutput.Re[k] *= gain
		linearOutput.Im[k] *= gain
	}
}
