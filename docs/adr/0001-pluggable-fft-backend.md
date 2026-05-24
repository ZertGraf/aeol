# ADR-0001: Pluggable FFT backend

## Status

Accepted

## Context

FFT is the dominant performance bottleneck in the library — the pure Go Ooura implementation is 9-12x slower than Rust's PFFFT (AVX2). Different consumers have different constraints: some need zero-CGO builds for easy cross-compilation and deployment, others want maximum throughput and are willing to accept a CGO dependency.

Currently `fft/` contains two concrete implementations (OouraFFT for 128-point, FFT4G for generic sizes) with no abstraction boundary. NS and AEC3 instantiate them directly.

## Decision

Define an FFT interface in the `fft` package that NS, AEC3, and any future consumer accept. Ship multiple implementations behind that interface:

- **Pure Go (Ooura)** — default, zero-CGO, works everywhere
- **CGO/PFFFT** — optional, links against PFFFT C library for AVX2/NEON acceleration
- **Possibly others** (FFTW, vDSP on macOS, etc.) as demand arises

The consumer chooses at construction time which backend to use. Each processing stage (NS Suppressor, AEC3 EchoCanceller3) accepts the FFT implementation as a constructor parameter or option.

## Consequences

- Processing stages become decoupled from FFT implementation — testable with a mock or a known-good reference
- Users who need zero-CGO get it by default with no build tags
- Users who need performance can opt in to a CGO backend
- The interface shape is load-bearing and hard to change after adoption — must be designed carefully up front
- Two implementations to maintain and test for correctness parity
