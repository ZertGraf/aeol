# Aeol — Architecture Reference

Go module: `aeol`. Zero external dependencies by default. CGO optional.

This document is a self-contained technical reference for AI assistants working with the codebase. It covers architecture, data flow, invariants, API surface, and build configurations.

---

## 1. What this library does

Real-time audio processing pipeline for voice communication. Ported from WebRTC M145 (Chromium). Stages: high-pass filter, acoustic echo cancellation, noise suppression, automatic gain control. All operate on 10 ms frames.

Not a streaming framework. Not an audio I/O layer. The caller provides frames, gets processed frames back.

---

## 2. Sample format invariant

**All internal processing uses FloatS16: `float32` in [-32768, 32767].**

This is the single most important fact. Every stage (NS, AEC3, AGC2, HPF) expects FloatS16. The top-level `AudioProcessing` wrapper auto-converts from:
- Normalized float32 [-1, 1] — multiplies by 32768
- Interleaved int16 — casts to float32

If you use standalone stages directly (e.g. `ns.Suppressor`), you must provide FloatS16 yourself.

---

## 3. Package map

```
aeol (root)           — orchestrator: AudioProcessing, Builder, Config, StreamConfig, AudioBuffer
  aec3/              — acoustic echo cancellation (adaptive filter, delay, suppression)
  agc2/              — automatic gain control v2 (adaptive + fixed gain)
  agc2/rnn_vad/      — RNN voice activity detector (pure Go, GRU network)
  ns/                — noise suppression (Wiener filter, spectral analysis)
  hpf/               — high-pass filter (biquad, removes <80 Hz)
  bands/             — frequency band splitting/merging for multi-rate
  dsp/               — splitting filter, biquad, sinc resampler, utilities
  fft/               — FFT interface + pure Go backends (Ooura radix-2, Bluestein chirp-z)
  fft/pffft/         — PFFFT CGO backend (optional, AVX2/NEON)
  rnnoise/           — RNNoise CGO wrapper (optional, neural denoising)
  simd/              — runtime SIMD backend selection (Scalar, Unrolled/AVX2)
  capi/              — C shared library exports (buildmode=c-shared)
  cmd/process_wav/   — batch WAV processor CLI
  cmd/measure_rms/   — RMS measurement CLI
```

---

## 4. Two usage modes

### 4.1 Orchestrated (recommended for most users)

```go
ap, _ := aeol.NewBuilder().
    SampleRate(48000).
    Channels(1).
    EnableNoiseSuppression(aeol.DefaultNsConfig()).
    EnableGainController2(aeol.DefaultGainController2Config()).
    Build()
defer ap.Close()

// every 10 ms:
ap.ProcessCaptureFloatNormalized([][]float32{frame})
```

The orchestrator handles: format conversion, band splitting (32/48 kHz -> 16 kHz sub-band), stage ordering, render routing for AEC, mutex, statistics.

### 4.2 Standalone stages (advanced)

```go
suppressor := ns.NewSuppressor(ns.Config{Level: ns.High})
// caller must provide 160 FloatS16 samples at 16 kHz
suppressor.Process(frame160)
```

Standalone stages: no format conversion, no band splitting, no mutex, no allocation beyond init. The caller owns the pipeline topology.

---

## 5. Processing pipeline order

```
Capture path (ProcessCapture*):
  PreAmplifier (linear gain)
  → CaptureLevelAdjustment.PreGainDb
  → HighPassFilter (per-channel biquad)
  → [Band split if rate > 16kHz]
  → AEC3.ProcessCapture (lower band only, 64-sample blocks)
  → NS.Process (lower band) + NS.ProcessUpperBand (upper bands)
  → [Band merge]
  → AGC2.Process (adaptive gain + limiter)
  → CaptureLevelAdjustment.PostGainDb
  → Statistics update

Render path (ProcessRender*):
  → [Band split if rate > 16kHz]
  → AEC3.ProcessRender (lower band only, 64-sample blocks)
```

Render MUST be called before the corresponding capture frame when AEC is enabled.

---

## 6. Frame sizes

| Sample rate | Frame (10 ms) | NS/AEC3 internal | AEC3 block |
|-------------|---------------|-------------------|------------|
| 16000 Hz    | 160 samples   | 160 samples       | 64 samples |
| 32000 Hz    | 320 samples   | 160 (lower band)  | 64 samples |
| 48000 Hz    | 480 samples   | 160 (lower band)  | 64 samples |

Band splitting: 32 kHz -> 2 bands, 48 kHz -> 3 bands. Only the lower band (0-8 kHz) is processed by NS and AEC3. Upper bands get optional attenuation from NS.

---

## 7. FFT system

Interface in `fft/`:
```go
type FFT interface {
    Forward(data []float32)  // in-place, packed format
    Inverse(data []float32)  // in-place, packed format
    Size() int
}
type Factory func(size int) FFT
```

Packed format after Forward: `[re[0], re[N/2], re[1], im[1], re[2], im[2], ...]`
DC and Nyquist are real-only, stored in first two slots.

Backends:
- `fft.DefaultFactory` — pure Go: Ooura (power-of-2) + Bluestein (arbitrary size)
- `fft/pffft.New` — CGO, wraps PFFFT C library with AVX2/NEON

Stages accept `fft.Factory` as constructor option. Omit for default pure Go.

---

## 8. VAD implementations

All implement `agc2.VADAnalyzer`:
```go
type VADAnalyzer interface {
    Analyze(samples []float32) float32  // returns speech probability [0, 1]
    Reset()
}
```

| Implementation | Package | Deps | CPU | Quality |
|---|---|---|---|---|
| VoiceActivityDetector | agc2 | none | ~0 | low (RMS threshold + hangover) |
| GMMVoiceActivityDetector | agc2 | none | low | medium (6-band filterbank, adaptive noise) |
| RNNVADWrapper | agc2 | fft | ~0.5ms/frame | high (GRU network, 24 kHz) |
| VADAdapter | rnnoise | CGO | ~1ms/frame | high (neural, 48 kHz) |

AGC2 accepts VAD via variadic constructor arg: `agc2.NewGainController2(cfg, vadImpl)`.

---

## 9. SIMD system

Package `simd/` provides runtime-detected backends for non-FFT hot loops (sinc resampler coefficients, vector ops).

```go
type Backend interface {
    DotProduct(a, b []float32) float32
    ScaleAdd(dst []float32, src []float32, scale float32)
    // ...
}
```

Detection: amd64 with AVX2 → Unrolled (manually unrolled loops); otherwise Scalar. ARM64 currently maps to Scalar (NEON backend placeholder exists).

FFT backends handle their own SIMD internally (PFFFT has AVX2/NEON in C code).

---

## 10. Build configurations

### 10.1 Pure Go (default)

```bash
go build ./...
```

- FFT: Ooura + Bluestein (pure Go)
- SIMD: Scalar or Unrolled (auto-detected, no CGO)
- VAD: RMS / GMM / RNN VAD (all pure Go)
- Cross-compiles to any GOOS/GOARCH
- Zero external dependencies

### 10.2 With PFFFT (CGO)

```bash
CGO_ENABLED=1 go build ./...
```

- Adds `fft/pffft` backend (~3x faster FFT on AVX2 hardware)
- Requires C compiler
- PFFFT sources vendored in-tree
- Accelerates NS and AEC3 (FFT-dominant stages)

### 10.3 With RNNoise (CGO)

```bash
CGO_ENABLED=1 go build ./...
```

- Adds `rnnoise/` package: neural network noise suppression + VAD
- Requires `third_party/rnnoise/` vendored C sources
- Operates at 48 kHz, 480 samples/frame
- Alternative to the Wiener-based NS, not a complement

### 10.4 Full CGO (PFFFT + RNNoise)

Maximum performance configuration. Both CGO packages active.

### 10.5 C shared library

```bash
go build -buildmode=c-shared -o aeol.dll ./capi/
```

Exports the pipeline as C API for FFI consumers (Python, Rust, C++, etc.). Any of the above configurations can be compiled this way.

---

## 11. Key architectural decisions

1. **No global state.** Every stage instance is self-contained. No init(), no package-level mutables.
2. **No allocations on the hot path.** All buffers are pre-allocated at construction. Process methods are zero-alloc.
3. **Nil-pointer == disabled.** Config uses `*T` pointers; nil means "skip this stage".
4. **Per-channel instances.** Each channel gets its own NS/AEC3/AGC2/HPF instance. No cross-channel coupling (except stereo render routing in AEC).
5. **In-place processing.** All Process methods modify the input slice. The caller reads results from the same memory.
6. **Frame-synchronous.** One call = one 10 ms frame. No internal buffering across calls (except AEC3's internal 64-sample block accumulator).

---

## 12. Thread safety

- `AudioProcessing` (orchestrator): all methods are mutex-protected. Safe for concurrent use.
- Individual stages (`ns.Suppressor`, `aec3.EchoCanceller3`, `agc2.GainController2`): NOT safe for concurrent use. Caller must synchronize.
- `fft.FFT` instances: NOT safe for concurrent use. Create one per goroutine.

---

## 13. Testing

```bash
go test ./...          # unit tests
go test -bench=. ./... # benchmarks
go test -v -run TestE2E_ ./  # end-to-end timing
```

Tests are self-contained. No external fixtures, no network, no file I/O (except cmd/ tests that use temp files).

---

## 14. Common tasks for AI assistants

### Adding a new processing stage

1. Create package `aeol/<stage>/` with a `Process([]float32)` method (FloatS16 in, FloatS16 out)
2. Add config type to `config.go` with `*T` pointer field in `Config`
3. Add `Enable<Stage>` method to `builder.go`
4. Wire into `processCaptureFloatLocked` in `aeol.go` at the correct pipeline position
5. Add per-channel instantiation in `newAudioProcessing` and `ApplyConfig`

### Adding a new FFT backend

1. Implement `fft.FFT` interface (Forward, Inverse, Size)
2. Provide a factory function `func(size int) fft.FFT`
3. Consumers pass it to stage constructors that accept `fft.Factory`

### Adding a new VAD

1. Implement `agc2.VADAnalyzer` (Analyze, Reset)
2. Pass instance to `agc2.NewGainController2(cfg, myVAD)`

### Changing AEC3 internals

AEC3 processes in 64-sample blocks. The orchestrator slices 160-sample frames into blocks via a loop. Internal state: delay estimator, adaptive filter (NLMS), suppression gain. Config via `aec3.DefaultConfig()` or custom `aec3.EchoCanceller3Config`.

---

## 15. What NOT to assume

- Do NOT assume normalized [-1, 1] audio anywhere inside the pipeline. It's FloatS16 everywhere.
- Do NOT assume stages can process arbitrary frame sizes. It's always exactly `SampleRate / 100` samples.
- Do NOT assume AEC3 processes full 160-sample frames atomically. It uses 64-sample sub-blocks.
- Do NOT assume NS/AEC3 work at the input sample rate. They always work at 16 kHz (lower band).
- Do NOT assume the library does I/O. It's a pure computation library.
- Do NOT assume RNNoise replaces NS. They are alternatives with different tradeoffs.
- Do NOT add external dependencies. The pure Go path must remain zero-dep.
