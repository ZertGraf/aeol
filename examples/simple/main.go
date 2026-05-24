package main

import (
	"fmt"

	"aeol"
)

func main() {
	// Simple example demonstrating basic echo cancellation with aeol.
	ap, err := aeol.NewBuilder().
		SampleRate(48000).
		Channels(1).
		EnableEchoCanceller(aeol.DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(aeol.DefaultNsConfig()).
		EnableGainController2(aeol.DefaultGainController2Config()).
		Build()

	if err != nil {
		panic(err)
	}
	defer ap.Close()

	fmt.Println("Aeol processor created successfully")
}
