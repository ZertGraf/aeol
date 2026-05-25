package aeol

// AudioProcessingStats holds metrics computed during the most recent ProcessCapture call.
// pointer fields are nil when the corresponding stage is not active.
type AudioProcessingStats struct {
	OutputRmsDbfs             *float64
	VoiceDetected             *bool
	EchoReturnLossEnhancement *float64
	DelayMs                   *int
}
