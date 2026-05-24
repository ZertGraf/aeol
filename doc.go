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
//	// process 10ms frames (normalized [-1, 1] float)
//	ap.ProcessRenderFloatNormalized([][]float32{renderFrame})
//	ap.ProcessCaptureFloatNormalized([][]float32{captureFrame})
//
//	// or int16 interleaved
//	ap.ProcessRenderInt16(renderSamples)
//	ap.ProcessCaptureInt16(captureSamples)
//
//	// or FloatS16 [-32768, 32767] float
//	ap.ProcessRenderFloatS16([][]float32{renderFrame})
//	ap.ProcessCaptureFloatS16([][]float32{captureFrame})
//
// Instances are not safe for concurrent use; synchronization is the caller's responsibility.
package sonora
