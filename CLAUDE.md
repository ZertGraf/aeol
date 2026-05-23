# Sonora (Go port)

Go port of the WebRTC-based audio processing pipeline (AEC3 + Noise Suppression + AGC2).

## Project structure

```
sonora.go          — main AudioProcessing processor, capture/render paths
builder.go         — Builder pattern API
config.go          — configuration types and defaults
buffer.go          — AudioBuffer with frequency band splitting
aec3/              — acoustic echo cancellation (adaptive filter, delay estimator, suppression)
ns/                — noise suppression (Wiener filter, speech probability, FFT-based)
agc2/              — automatic gain control v2 (adaptive + fixed digital gain)
dsp/               — splitting filter, biquad, sinc resampler, utilities
fft/               — Ooura FFT (128-point for AEC3) and FFT4G (generic sizes for NS)
simd/              — runtime-selectable SIMD backends (Scalar, SSE2, AVX2, NEON)
```

## Build and test

```bash
go test ./...
go vet ./...
```

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
