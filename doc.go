// Package sonora provides real-time audio processing algorithms
// ported from the WebRTC native codebase (M145).
//
// Core capabilities:
//   - Acoustic Echo Cancellation (AEC3)
//   - Noise Suppression (NS)
//   - Automatic Gain Control (AGC2)
//   - High-Pass Filtering
//   - Voice Activity Detection (VAD)
//
// Basic usage:
//
//	ap, err := sonora.NewBuilder().
//		SampleRate(48000).
//		Channels(1).
//		EnableEchoCanceller(sonora.DefaultEchoCancellerConfig()).
//		EnableNoiseSuppression(sonora.DefaultNsConfig()).
//		Build()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer ap.Close()
//
//	// process 10ms frames
//	ap.ProcessCapture(captureFrame)
//	ap.ProcessRender(renderFrame)
package sonora
