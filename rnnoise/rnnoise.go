// Package rnnoise provides a CGO wrapper around the RNNoise library
// (https://github.com/xiph/rnnoise) for neural-network-based noise suppression.
//
// RNNoise uses GRU layers trained on speech/noise corpora to suppress noise
// while preserving speech. It operates at 48 kHz on 480-sample (10 ms) frames
// in FloatS16 format (float32 in [-32768, 32767]).
//
// Requires CGO and a C compiler. Vendor the C sources first:
//
//	cd third_party/rnnoise && ./vendor.sh   # or .\vendor.ps1 on Windows
//
// Usage:
//
//	d := rnnoise.New()
//	defer d.Close()
//
//	vadProb := d.ProcessFrame(frame480)  // 480 FloatS16 samples, in-place
//
// Each instance is single-channel. For stereo, create two instances.
// Instances are not safe for concurrent use.
package rnnoise

/*
#cgo CFLAGS: -I${SRCDIR}/../third_party/rnnoise/include -I${SRCDIR}/../third_party/rnnoise/src -O3
#cgo LDFLAGS: -lm
#include "rnnoise.h"
*/
import "C"

import (
	"runtime"
	"unsafe"
)

const (
	// FrameSize is the number of samples per frame (10 ms at 48 kHz).
	FrameSize = 480
	// SampleRate is the native sample rate of RNNoise.
	SampleRate = 48000
)

// Denoiser wraps a single-channel RNNoise instance.
type Denoiser struct {
	state *C.DenoiseState
	inBuf [FrameSize]C.float
	outBuf [FrameSize]C.float
}

// New creates a new RNNoise denoiser using the built-in model.
func New() *Denoiser {
	d := &Denoiser{
		state: C.rnnoise_create(nil),
	}
	runtime.SetFinalizer(d, (*Denoiser).release)
	return d
}

func (d *Denoiser) release() {
	if d.state != nil {
		C.rnnoise_destroy(d.state)
		d.state = nil
	}
}

// ProcessFrame suppresses noise in a 480-sample FloatS16 frame in-place.
// Returns the VAD (voice activity) probability in [0, 1].
// Panics if len(frame) < 480.
func (d *Denoiser) ProcessFrame(frame []float32) float32 {
	_ = frame[FrameSize-1]

	in := unsafe.Slice((*float32)(unsafe.Pointer(&d.inBuf[0])), FrameSize)
	out := unsafe.Slice((*float32)(unsafe.Pointer(&d.outBuf[0])), FrameSize)

	copy(in, frame[:FrameSize])
	vad := C.rnnoise_process_frame(d.state, &d.outBuf[0], &d.inBuf[0])
	copy(frame[:FrameSize], out)

	runtime.KeepAlive(d)
	return float32(vad)
}

// Reset destroys and recreates the internal state, clearing all history.
func (d *Denoiser) Reset() {
	if d.state != nil {
		C.rnnoise_destroy(d.state)
	}
	d.state = C.rnnoise_create(nil)
}

// Close releases C resources immediately. The instance must not be used after.
func (d *Denoiser) Close() {
	runtime.SetFinalizer(d, nil)
	d.release()
}

// VADAdapter wraps a Denoiser to satisfy the agc2.VADAnalyzer interface.
// It resamples 160-sample 16 kHz frames to 48 kHz internally using linear
// interpolation (sufficient for VAD energy features), runs RNNoise, and
// returns the VAD probability. The denoised audio is discarded.
//
// Usage:
//
//	gc := agc2.NewGainController2(cfg, rnnoise.NewVADAdapter())
type VADAdapter struct {
	denoiser *Denoiser
	buf48    [FrameSize]float32
}

// NewVADAdapter creates a VADAdapter with its own Denoiser instance.
func NewVADAdapter() *VADAdapter {
	return &VADAdapter{
		denoiser: New(),
	}
}

// Analyze resamples the 160-sample 16 kHz frame to 48 kHz, runs RNNoise
// (discarding the denoised output), and returns the VAD probability [0, 1].
func (va *VADAdapter) Analyze(samples []float32) float32 {
	n := len(samples)
	if n == 0 {
		return 0
	}

	// linear interpolation: 160 samples @ 16kHz -> 480 samples @ 48kHz (3x)
	ratio := float32(n) / float32(FrameSize)
	for i := 0; i < FrameSize; i++ {
		srcIdx := float32(i) * ratio
		idx := int(srcIdx)
		frac := srcIdx - float32(idx)
		if idx >= n-1 {
			va.buf48[i] = samples[n-1]
		} else {
			va.buf48[i] = samples[idx]*(1-frac) + samples[idx+1]*frac
		}
	}

	return va.denoiser.ProcessFrame(va.buf48[:])
}

// Reset clears the internal denoiser state.
func (va *VADAdapter) Reset() {
	va.denoiser.Reset()
}

// Close releases the underlying Denoiser resources.
func (va *VADAdapter) Close() {
	va.denoiser.Close()
}
