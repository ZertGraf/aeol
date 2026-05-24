// Package pffft provides a SIMD-optimized FFT backend using PFFFT
// (Pretty Fast FFT by Julien Pommier, BSD license).
//
// Requires CGO and a C compiler (gcc/clang). Uses SSE on x86, NEON on ARM,
// scalar fallback otherwise. Typically 5-10x faster than the pure Go FFT
// for the sizes used by NS (256) and AEC3 (128).
//
// Drop-in replacement for the default Go backend:
//
//	suppressor := ns.NewSuppressor(cfg, pffft.Factory)
//	ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), rate, 1, pffft.Factory)
//
// Instances are not safe for concurrent use.
package pffft

/*
#cgo CFLAGS: -O3
#cgo LDFLAGS: -lm
#include "pffft.h"
*/
import "C"

import (
	"runtime"
	"unsafe"

	"sonora/fft"
)

// FFT wraps a PFFFT real-valued transform of size N.
type FFT struct {
	setup *C.PFFFT_Setup
	n     int
	buf   *C.float
	out   *C.float
}

// New creates a PFFFT instance. Size must be (2^a)*(3^b)*(5^c), a >= 5.
// Minimum size is 32.
func New(size int) *FFT {
	setup := C.pffft_new_setup(C.int(size), C.PFFFT_REAL)
	if setup == nil {
		panic("pffft: unsupported size")
	}
	f := &FFT{
		setup: setup,
		n:     size,
		buf:   (*C.float)(C.pffft_aligned_malloc(C.size_t(size * 4))),
		out:   (*C.float)(C.pffft_aligned_malloc(C.size_t(size * 4))),
	}
	runtime.SetFinalizer(f, (*FFT).release)
	return f
}

func (f *FFT) release() {
	if f.setup != nil {
		C.pffft_destroy_setup(f.setup)
		C.pffft_aligned_free(unsafe.Pointer(f.buf))
		C.pffft_aligned_free(unsafe.Pointer(f.out))
		f.setup = nil
	}
}

func (f *FFT) Size() int { return f.n }

// Forward computes the real FFT in packed format.
// Output: [re[0], re[N/2], re[1], im[1], re[2], im[2], ...].
func (f *FFT) Forward(data []float32) {
	if len(data) < f.n {
		return
	}
	n := f.n
	in := unsafe.Slice((*float32)(unsafe.Pointer(f.buf)), n)
	out := unsafe.Slice((*float32)(unsafe.Pointer(f.out)), n)
	copy(in, data[:n])
	C.pffft_transform_ordered(f.setup, f.buf, f.out, nil, C.PFFFT_FORWARD)
	copy(data[:n], out)
	runtime.KeepAlive(f)
}

// Inverse computes the inverse real FFT with 1/N normalization,
// matching the Go FFT convention (Forward then Inverse = identity).
func (f *FFT) Inverse(data []float32) {
	if len(data) < f.n {
		return
	}
	n := f.n
	in := unsafe.Slice((*float32)(unsafe.Pointer(f.buf)), n)
	out := unsafe.Slice((*float32)(unsafe.Pointer(f.out)), n)
	copy(in, data[:n])
	C.pffft_transform_ordered(f.setup, f.buf, f.out, nil, C.PFFFT_BACKWARD)
	s := float32(1.0) / float32(n)
	for i := 0; i < n; i++ {
		data[i] = out[i] * s
	}
	runtime.KeepAlive(f)
}

// Close releases C resources immediately. The instance must not be used after.
func (f *FFT) Close() {
	runtime.SetFinalizer(f, nil)
	f.release()
}

// Factory creates a PFFFT-backed FFT. Use as fft.Factory argument.
func Factory(size int) fft.FFT {
	return New(size)
}
