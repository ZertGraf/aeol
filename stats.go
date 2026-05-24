package aeol

type AudioProcessingStats struct {
	OutputRmsDbfs                   *float64
	VoiceDetected                   *bool
	EchoReturnLoss                  *float64
	EchoReturnLossEnhancement       *float64
	ResidualEchoLikelihood          *float64
	DelayMs                         *int
	DelayMedianMs                   *int
	DelayStandardDeviationMs        *int
	DivergentFilterFraction         *float64
}
