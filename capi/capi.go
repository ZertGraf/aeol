package main

/*
#include <stdint.h>
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"aeol/aec3"
	agc "aeol/agc2"
	"aeol/bands"
	"aeol/hpf"
	"aeol/ns"
)

// handle registry — never panics on invalid handles, thread-safe.
var (
	registry sync.Map
	seq      atomic.Uintptr
)

func reg(v any) C.uintptr_t {
	id := seq.Add(1)
	registry.Store(id, v)
	return C.uintptr_t(id)
}

func get[T any](h C.uintptr_t) (T, bool) {
	v, ok := registry.Load(uintptr(h))
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

func del(h C.uintptr_t) {
	registry.Delete(uintptr(h))
}

func cslice(p *C.float, n C.int) []float32 {
	if p == nil || n <= 0 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(p)), int(n))
}

// ---------------------------------------------------------------------------
// noise suppressor
// ---------------------------------------------------------------------------

//export aeol_ns_create
func aeol_ns_create(level C.int) C.uintptr_t {
	cfg := ns.Config{Level: ns.SuppressionLevel(level)}
	return reg(ns.NewSuppressor(cfg))
}

//export aeol_ns_process
func aeol_ns_process(h C.uintptr_t, data *C.float, n C.int) {
	s, ok := get[*ns.Suppressor](h)
	if !ok {
		return
	}
	s.Process(cslice(data, n))
}

//export aeol_ns_process_upper_band
func aeol_ns_process_upper_band(h C.uintptr_t, data *C.float, n C.int, band C.int) {
	s, ok := get[*ns.Suppressor](h)
	if !ok {
		return
	}
	s.ProcessUpperBand(cslice(data, n), int(band))
}

//export aeol_ns_reset
func aeol_ns_reset(h C.uintptr_t) {
	if s, ok := get[*ns.Suppressor](h); ok {
		s.Reset()
	}
}

//export aeol_ns_destroy
func aeol_ns_destroy(h C.uintptr_t) { del(h) }

// ---------------------------------------------------------------------------
// echo canceller (aec3)
// ---------------------------------------------------------------------------

//export aeol_aec3_create
func aeol_aec3_create(sampleRate C.uint32_t) C.uintptr_t {
	return reg(aec3.NewEchoCanceller3(aec3.DefaultConfig(), uint32(sampleRate), 1))
}

//export aeol_aec3_process_render
func aeol_aec3_process_render(h C.uintptr_t, data *C.float, n C.int) {
	ec, ok := get[*aec3.EchoCanceller3](h)
	if !ok {
		return
	}
	frame := cslice(data, n)
	for off := 0; off+aec3.BlockSize <= len(frame); off += aec3.BlockSize {
		ec.ProcessRender(frame[off : off+aec3.BlockSize])
	}
}

//export aeol_aec3_process_capture
func aeol_aec3_process_capture(h C.uintptr_t, data *C.float, n C.int) {
	ec, ok := get[*aec3.EchoCanceller3](h)
	if !ok {
		return
	}
	frame := cslice(data, n)
	for off := 0; off+aec3.BlockSize <= len(frame); off += aec3.BlockSize {
		ec.ProcessCapture(frame[off : off+aec3.BlockSize])
	}
}

//export aeol_aec3_erle
func aeol_aec3_erle(h C.uintptr_t) C.float {
	ec, ok := get[*aec3.EchoCanceller3](h)
	if !ok {
		return 1
	}
	return C.float(ec.ERLE())
}

//export aeol_aec3_delay
func aeol_aec3_delay(h C.uintptr_t) C.int {
	ec, ok := get[*aec3.EchoCanceller3](h)
	if !ok {
		return 0
	}
	return C.int(ec.Delay())
}

//export aeol_aec3_reset
func aeol_aec3_reset(h C.uintptr_t) {
	if ec, ok := get[*aec3.EchoCanceller3](h); ok {
		ec.Reset()
	}
}

//export aeol_aec3_destroy
func aeol_aec3_destroy(h C.uintptr_t) { del(h) }

// ---------------------------------------------------------------------------
// agc2
// ---------------------------------------------------------------------------

//export aeol_agc2_create
func aeol_agc2_create() C.uintptr_t {
	return reg(agc.NewGainController2(agc.DefaultConfig()))
}

//export aeol_agc2_create_ex
func aeol_agc2_create_ex(fixedGainDb C.float, enableAdaptive C.int) C.uintptr_t {
	cfg := agc.DefaultConfig()
	cfg.FixedDigital.GainDb = float32(fixedGainDb)
	cfg.AdaptiveDigital.Enabled = enableAdaptive != 0
	return reg(agc.NewGainController2(cfg))
}

//export aeol_agc2_process
func aeol_agc2_process(h C.uintptr_t, data *C.float, n C.int) {
	gc, ok := get[*agc.GainController2](h)
	if !ok {
		return
	}
	gc.Process(cslice(data, n))
}

//export aeol_agc2_reset
func aeol_agc2_reset(h C.uintptr_t) {
	if gc, ok := get[*agc.GainController2](h); ok {
		gc.Reset()
	}
}

//export aeol_agc2_destroy
func aeol_agc2_destroy(h C.uintptr_t) { del(h) }

// ---------------------------------------------------------------------------
// high-pass filter
// ---------------------------------------------------------------------------

//export aeol_hpf_create
func aeol_hpf_create(sampleRate C.uint32_t) C.uintptr_t {
	return reg(hpf.New(uint32(sampleRate)))
}

//export aeol_hpf_process
func aeol_hpf_process(h C.uintptr_t, data *C.float, n C.int) {
	f, ok := get[*hpf.Filter](h)
	if !ok {
		return
	}
	f.Process(cslice(data, n))
}

//export aeol_hpf_reset
func aeol_hpf_reset(h C.uintptr_t) {
	if f, ok := get[*hpf.Filter](h); ok {
		f.Reset()
	}
}

//export aeol_hpf_destroy
func aeol_hpf_destroy(h C.uintptr_t) { del(h) }

// ---------------------------------------------------------------------------
// band splitter
// ---------------------------------------------------------------------------

//export aeol_bands_create
func aeol_bands_create(sampleRate C.uint32_t) C.uintptr_t {
	return reg(bands.New(uint32(sampleRate)))
}

//export aeol_bands_count
func aeol_bands_count(h C.uintptr_t) C.int {
	sp, ok := get[*bands.Splitter](h)
	if !ok {
		return 1
	}
	return C.int(sp.Bands())
}

//export aeol_bands_frame_len
func aeol_bands_frame_len(h C.uintptr_t) C.int {
	sp, ok := get[*bands.Splitter](h)
	if !ok {
		return 0
	}
	return C.int(sp.FrameLength())
}

//export aeol_bands_band_len
func aeol_bands_band_len(h C.uintptr_t) C.int {
	sp, ok := get[*bands.Splitter](h)
	if !ok {
		return 0
	}
	return C.int(sp.BandLength())
}

//export aeol_bands_split
func aeol_bands_split(h C.uintptr_t, frame *C.float, lowerOut *C.float, upperOut *C.float) {
	sp, ok := get[*bands.Splitter](h)
	if !ok {
		return
	}
	fLen := sp.FrameLength()
	bLen := sp.BandLength()
	nBands := sp.Bands()

	lower, upper := sp.Split(cslice(frame, C.int(fLen)))
	copy(cslice(lowerOut, C.int(bLen)), lower)
	if nBands > 1 && upperOut != nil {
		dst := cslice(upperOut, C.int((nBands-1)*bLen))
		for i, ub := range upper {
			copy(dst[i*bLen:(i+1)*bLen], ub)
		}
	}
}

//export aeol_bands_merge
func aeol_bands_merge(h C.uintptr_t, lowerIn *C.float, upperIn *C.float, frameOut *C.float) {
	sp, ok := get[*bands.Splitter](h)
	if !ok {
		return
	}
	fLen := sp.FrameLength()
	bLen := sp.BandLength()
	nBands := sp.Bands()

	lowerSlice := cslice(lowerIn, C.int(bLen))
	var upperSlices [][]float32
	if nBands > 1 && upperIn != nil {
		flat := cslice(upperIn, C.int((nBands-1)*bLen))
		upperSlices = make([][]float32, nBands-1)
		for i := range upperSlices {
			upperSlices[i] = flat[i*bLen : (i+1)*bLen]
		}
	}
	sp.Merge(lowerSlice, upperSlices, cslice(frameOut, C.int(fLen)))
}

//export aeol_bands_reset
func aeol_bands_reset(h C.uintptr_t) {
	if sp, ok := get[*bands.Splitter](h); ok {
		sp.Reset()
	}
}

//export aeol_bands_destroy
func aeol_bands_destroy(h C.uintptr_t) { del(h) }

func main() {}
