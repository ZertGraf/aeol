# Aeol (Go port)

Go port of the WebRTC-based audio processing pipeline (M145). Module path: `aeol`. Go 1.23+. Zero external dependencies; CGO optional (pffft, rnnoise).

## Project structure

```
aeol.go            — main AudioProcessing, capture/render paths
builder.go         — Builder pattern API
config.go          — configuration types and defaults
stream.go          — StreamConfig, format conversion helpers
buffer.go          — AudioBuffer with frequency band splitting
stats.go           — processing statistics
high_pass_filter.go — HPF integrated into main pipeline
doc.go             — package doc comment
aec3/              — acoustic echo cancellation (adaptive filter, delay estimator, suppression)
ns/                — noise suppression (Wiener filter, speech probability, FFT-based)
agc2/              — automatic gain control v2 (adaptive + fixed digital gain)
agc2/rnn_vad/      — RNN-based voice activity detection (pure Go, ported from WebRTC)
hpf/               — high-pass filter (standalone package)
bands/             — frequency band splitting (standalone package)
dsp/               — splitting filter, biquad, sinc resampler, utilities
fft/               — FFT interface + pure Go Ooura backend
fft/pffft/         — PFFFT CGO backend (optional, requires C compiler)
rnnoise/           — RNNoise CGO wrapper (optional, requires C compiler)
third_party/       — vendored C sources (rnnoise)
simd/              — runtime-selectable SIMD backends (Scalar, Unrolled, NEON)
capi/              — C shared library exports (buildmode=c-shared)
cmd/process_wav/   — batch WAV processing utility
cmd/measure_rms/   — RMS measurement utility
examples/simple/   — minimal usage example
```

## Build and test

```bash
go test ./...
go vet ./...
```

## Gotchas

- module path is `aeol`, not the directory name
- all internal processing uses FloatS16: float32 in [-32768, 32767], NOT normalized [-1, 1]
- pipeline wrapper auto-converts from normalized/int16, standalone stages expect FloatS16 directly
- NS and AEC3 operate at 16 kHz internally; for 32/48 kHz use `bands.Splitter` to split/merge
- AEC3 processes 64-sample sub-blocks internally (FrameBlocker), but the API accepts 160-sample frames
- FFT packed format: [re[0], re[N/2], re[1], im[1], re[2], im[2], ...] — DC and Nyquist in first two slots
- `fft.Factory` is passed as variadic option to NS/AEC3/RNN VAD constructors; omit for default pure Go backend
- no stage is safe for concurrent use; AudioProcessing wrapper has a mutex, standalone stages do not
- RNN VAD operates at 24 kHz; the RNNVADWrapper in agc2 handles resampling from other rates
- rnnoise/ requires CGO and vendored C sources in third_party/rnnoise/

## VAD options

Three VAD implementations with different tradeoffs:
- `agc2.VoiceActivityDetector` — RMS-based, zero-alloc, minimal CPU (default)
- `agc2.RNNVADWrapper` → `agc2/rnn_vad` — pure Go port of WebRTC's RNN VAD (GRU network, ~0.5ms/frame)
- `agc2.GMMVoiceActivityDetector` — GMM-based with 6-band filterbank, adaptive noise model
- `rnnoise.VADAdapter` — CGO wrapper around RNNoise (48kHz, requires C compiler)

## Benchmarks

### Go benchmarks (per-stage and full pipeline)

```bash
# all benchmarks with memory stats
go test -bench=. -benchmem -run=^$ -count=3

# specific stage
go test -bench=BenchmarkStage_AEC3Capture -benchmem -run=^$

# full pipeline only
go test -bench=BenchmarkPipeline -benchmem -run=^$ -count=3
```

### E2E timing tests (human-readable report with percentiles)

```bash
# all e2e timing scenarios
go test -v -run "TestE2E_" -count=1

# specific sample rate
go test -v -run "TestE2E_StageTiming_16kHz" -count=1
go test -v -run "TestE2E_StageTiming_48kHz" -count=1

# throughput (frames/sec, realtime multiplier)
go test -v -run "TestE2E_Throughput" -count=1
```

### Rust reference (for comparison on the same machine)

The original Rust implementation lives at `D:\Trash\sonora`. To run its criterion benchmarks:

```bash
cd D:\Trash\sonora
RUSTFLAGS="-C target-cpu=native" cargo bench -p sonora --bench pipeline
```

This produces results for `process_stream/{16k_mono,48k_mono,48k_stereo}`, `noise_suppressor/analyze_and_process`, `pffft/forward_{128,256,512}`, and `sinc_resampler/48k_to_16k`.

### Performance reference (AMD Ryzen 7 7735U, 2026-05-22)

| benchmark | Rust | Go | Go/Rust |
|---|---|---|---|
| 16kHz mono full pipeline | 6.25 µs | 75.3 µs | 12.0x |
| 48kHz mono full pipeline | 20.3 µs | 97.4 µs | 4.8x |
| 48kHz stereo full pipeline | 38.7 µs | 189.7 µs | 4.9x |
| NS only (16kHz) | 1.53 µs | 14.6 µs | 9.5x |
| AGC2 only (16kHz) | ~0.5 µs | 778 ns | 1.6x |

All configurations comfortably real-time: full pipeline uses <1% of 10ms frame budget.

Methodology difference: Rust `process_stream` measures capture-only; Go `BenchmarkPipeline_Full` measures render+capture.

### Known performance gaps

1. **AEC3 adaptive filter** — scalar inner loop; SIMD dispatch was removed, re-adding a SIMD-backed filter is the next optimization target
2. **FFT** — pure Go Ooura vs Rust PFFFT (AVX2-optimized); 446ns vs ~2-3µs per 128-point transform
3. **NS** — same FFT bottleneck on 512-point transforms

### Benchmark file

`benchmark_e2e_test.go` contains:
- `TestE2E_StageTiming_{16kHz,48kHz}` — batched per-stage timing with percentiles
- `TestE2E_MultiChannel` — 2-channel 48kHz pipeline
- `TestE2E_BandSplitting` — splitting filter overhead by sample rate
- `TestE2E_AEC3Convergence` — AEC3 timing during adaptation
- `TestE2E_NSLevels` — NS timing across suppression levels
- `TestE2E_Int16VsFloat32` — format conversion overhead
- `TestE2E_Throughput` — frames/sec across configurations
- `TestE2E_SplittingFilter` — isolated analysis+synthesis round-trip
- `BenchmarkStage_*` — individual stage benchmarks (testing.B)
- `BenchmarkPipeline_*` — full pipeline benchmarks (testing.B)
