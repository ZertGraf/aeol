# Aeol

Real-time audio processing toolkit in pure Go. Implements acoustic echo cancellation, noise suppression, automatic gain control, and high-pass filtering — ported from the WebRTC native codebase (M145).

Designed as a **toolkit of independent stages**, not a monolithic pipeline. Each stage is importable and usable on its own. How to combine and order stages is the caller's responsibility.

## Features

| Stage | Package | Description |
|-------|---------|-------------|
| Noise Suppression | `ns` | Wiener filter with speech probability estimation, 4 suppression levels |
| Echo Cancellation | `aec3` | Adaptive filter with delay estimation, NLMS subtractor |
| Gain Control | `agc2` | Adaptive + fixed digital gain with level limiter |
| High-Pass Filter | `hpf` | Cascaded biquad, removes DC and sub-80Hz rumble |
| Band Splitting | `bands` | QMF analysis/synthesis for multi-rate processing |
| FFT | `fft` | Pluggable backend: pure Go (default) or PFFFT via CGO |

## Install

The module path will change when the library moves to its public repository.
For now, use a `replace` directive in your `go.mod`:

```
require sonora v0.0.0
replace sonora => ../path/to/sonora
```

No CGO required. Zero external dependencies.

## Quick Start

### Pipeline mode (convenience wrapper)

```go
ap, _ := sonora.NewBuilder().
    SampleRate(48000).
    Channels(1).
    EnableNoiseSuppression(sonora.DefaultNsConfig()).
    EnableEchoCanceller(sonora.DefaultEchoCancellerConfig()).
    EnableGainController2(sonora.DefaultGainController2Config()).
    Build()
defer ap.Close()

// 10ms frames, normalized float [-1, 1]
ap.ProcessRenderFloatNormalized([][]float32{renderFrame})
ap.ProcessCaptureFloatNormalized([][]float32{captureFrame})
```

### Standalone stages

```go
import "sonora/ns"

suppressor := ns.NewSuppressor(ns.Config{Level: ns.High})

// 160 samples = 10ms at 16kHz, FloatS16 range
suppressor.Process(frame)
```

```go
import "sonora/aec3"

ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), 16000, 1)

// 64-sample blocks
ec.ProcessRender(renderBlock)
ec.ProcessCapture(captureBlock)
```

```go
import "sonora/agc2"

gc := agc2.NewGainController2(agc2.DefaultConfig())
gc.Process(frame) // any frame size
```

### Multi-rate processing with band splitting

NS and AEC3 operate at 16 kHz internally. For higher sample rates, split into bands first:

```go
import "sonora/bands"

sp := bands.New(48000) // 3 bands for 48 kHz

lower, upper := sp.Split(frame480)
suppressor.Process(lower)
for i, ub := range upper {
    suppressor.ProcessUpperBand(ub, i)
}
sp.Merge(lower, upper, frame480)
```

## Sample Format

All internal processing uses **FloatS16**: `float32` values in `[-32768, 32767]`. This matches the WebRTC convention.

The pipeline wrapper accepts three formats:
- `ProcessCaptureFloatNormalized` / `ProcessRenderFloatNormalized` — `[-1, 1]` float (converts automatically)
- `ProcessCaptureFloatS16` / `ProcessRenderFloatS16` — `[-32768, 32767]` float (native, no conversion)
- `ProcessCaptureInt16` / `ProcessRenderInt16` — interleaved `int16`

Conversion helpers:
```go
sonora.ToFloatS16(samples)   // [-1,1] -> [-32768, 32767]
sonora.FromFloatS16(samples) // [-32768, 32767] -> [-1,1]
```

## FFT Backend

The default FFT is a pure Go Ooura implementation. For 5-10x faster transforms, use the PFFFT backend (requires CGO + C compiler):

```go
import "sonora/fft/pffft"

suppressor := ns.NewSuppressor(cfg, pffft.Factory)
ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), rate, 1, pffft.Factory)
```

PFFFT uses SSE on x86, NEON on ARM, scalar fallback otherwise.

## RNNoise Backend

Neural-network-based noise suppression via [RNNoise](https://github.com/xiph/rnnoise) (requires CGO + C compiler). Operates at 48 kHz on 480-sample frames. Also provides a VAD probability as a bonus output.

```bash
# vendor C sources (one-time)
cd third_party/rnnoise && ./vendor.sh  # or .\vendor.ps1 on Windows
```

```go
import "sonora/rnnoise"

d := rnnoise.New()
defer d.Close()

vadProb := d.ProcessFrame(frame480)  // 480 FloatS16 samples, in-place
```

RNNoise VAD can also feed AGC2 via the adapter:

```go
gc := agc2.NewGainController2(cfg, rnnoise.NewVADAdapter())
```

## C API (FFI)

Build as a shared library for use from C, Python, Rust, etc.:

```bash
go build -buildmode=c-shared -o libsonora.so ./capi/
```

See [`capi/sonora.h`](capi/sonora.h) for the full API. Each stage is independent — create one handle per channel:

```c
#include "sonora.h"

sonora_handle ns = sonora_ns_create(2); // level=high
sonora_ns_process(ns, samples, 160);
sonora_ns_destroy(ns);
```

## Project Structure

```
sonora.go        main AudioProcessing, capture/render paths
builder.go       Builder pattern API
config.go        configuration types and defaults
stream.go        StreamConfig, format conversion helpers
buffer.go        AudioBuffer with frequency band splitting
aec3/            acoustic echo cancellation
ns/              noise suppression
agc2/            automatic gain control
hpf/             high-pass filter (standalone)
bands/           frequency band splitting (standalone)
dsp/             splitting filter, biquad, sinc resampler
fft/             FFT interface + pure Go backend
fft/pffft/       PFFFT CGO backend (optional)
rnnoise/         RNNoise CGO wrapper (optional)
third_party/     vendored C sources (pffft, rnnoise)
simd/            runtime-selectable SIMD backends
capi/            C shared library exports
cmd/process_wav  batch WAV processing utility
cmd/measure_rms  RMS measurement utility
```

## Build & Test

```bash
go test ./...
go vet ./...
```

Benchmarks:

```bash
go test -bench=. -benchmem -run=^$ -count=3
```

## Performance

Measured on AMD Ryzen 7 7735U (Go 1.23, pure Go FFT):

| Configuration | Time per frame | Realtime budget used |
|--------------|---------------|---------------------|
| 16 kHz mono, full pipeline | 75 us | 0.75% of 10ms |
| 48 kHz mono, full pipeline | 97 us | 0.97% of 10ms |
| 48 kHz stereo, full pipeline | 190 us | 1.9% of 10ms |

All configurations comfortably real-time. With PFFFT backend, expect 3-5x improvement on FFT-bound stages (NS, AEC3).

## Thread Safety

Individual stages (`ns.Suppressor`, `aec3.EchoCanceller3`, `agc2.GainController2`) are **not** safe for concurrent use. They are stateful library primitives. Synchronization is the caller's responsibility.

The top-level `AudioProcessing` wrapper provides a mutex for convenience.

## License

BSD-3-Clause. PFFFT backend includes code by Julien Pommier (BSD/FFTPACK license).
