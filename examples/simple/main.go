// Simple example demonstrating basic echo cancellation with sonora.
package main

import (
	"fmt"
	"math"

	"sonora"
)

func main() {
	ap, err := sonora.NewBuilder().
		SampleRate(48000).
		Channels(1).
		EnableEchoCanceller(sonora.DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(sonora.DefaultNsConfig()).
		EnableGainController2(sonora.DefaultGainController2Config()).
		Build()
	if err != nil {
		panic(err)
	}
	defer ap.Close()

	frameSize := 480
	renderFrame := make([]float32, frameSize)
	captureFrame := make([]float32, frameSize)

	for iter := 0; iter < 500; iter++ {
		for i := range renderFrame {
			t := float64(iter*frameSize+i) / 48000.0
			renderFrame[i] = 0.5 * float32(math.Sin(2*math.Pi*440*t))
		}

		if err := ap.ProcessRenderFloatNormalized([][]float32{renderFrame}); err != nil {
			panic(err)
		}

		for i := range captureFrame {
			captureFrame[i] = 0.3 * renderFrame[i]
			captureFrame[i] += 0.001 * float32(math.Sin(2*math.Pi*1000*float64(iter*frameSize+i)/48000.0))
		}

		if err := ap.ProcessCaptureFloatNormalized([][]float32{captureFrame}); err != nil {
			panic(err)
		}
	}

	stats := ap.Statistics()
	if stats.OutputRmsDbfs != nil {
		fmt.Printf("output RMS: %.2f dBFS\n", *stats.OutputRmsDbfs)
	}
	if stats.EchoReturnLossEnhancement != nil {
		fmt.Printf("ERLE: %.2f dB\n", *stats.EchoReturnLossEnhancement)
	}
	fmt.Println("done: processed 500 frames with AEC + NS + AGC")
}
