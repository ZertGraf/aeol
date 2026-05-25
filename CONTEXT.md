# Ubiquitous Language

## Aeol
Permanent name for the library. Short for Aeolus (god of wind). Independent Go module (`aeol`). Inspired by WebRTC M145 algorithms, not bit-exact compatible. Quality target: subjectively no worse than the Rust reference on the same recordings.

This is a **toolkit of independent audio processing stages**. Each stage (NS, AEC3, AGC2, HPF) is importable and usable on its own via subpackages. Standalone stages operate on raw FloatS16 frames at 16kHz — no format conversion, no band splitting, no synchronization. The top-level `AudioProcessing` struct is an **orchestrator** for the typical use case: it handles format conversion (normalized/int16 ↔ FloatS16), frequency band splitting/merging for multi-rate input, correct stage ordering, stereo render routing, and mutex-based thread safety. Use standalone stages when you need a non-standard pipeline (e.g. NS on raw 16kHz without the overhead of splitting).

## Capture
Audio coming from the microphone (near-end signal). The signal to be cleaned up.

## Render
Audio coming from the far end (speaker playback). Used by AEC3 as a reference to identify and remove echo.

## FloatS16
Sample representation used by all processing stages: float32 values in the range [-32768, 32767]. Matches WebRTC's "FloatS16" convention. Individual stages (ns, aec3, agc2) accept and return FloatS16 exclusively. Format conversion is the caller's responsibility; utility functions are provided in the root package.

## Statistics
AudioProcessingStats exposes metrics computed by the orchestrator: output RMS level, VAD speech detection, ERLE, and delay estimate. Pointer fields are nil when the corresponding stage is not active. Individual stages also export their own metrics directly (ERLE, GainDb, SpeechProbability, etc.) for standalone use.

## Thread safety
Individual processing stages are not safe for concurrent use. They are stateful library primitives, not services. Synchronization is the caller's responsibility. The top-level AudioProcessing wrapper provides a mutex as convenience.

## AEC3
Acoustic Echo Cancellation engine. Operates on the 0-8kHz band only (16kHz internal rate); upper frequency bands are not echo-cancelled. Uses a dual adaptive filter (refined + coarse NLMS) with delay estimation and frequency-domain suppression. Processes 64-sample blocks. The orchestrator handles band splitting and block slicing automatically.

## Band splitting
Frequency band splitting for multi-rate processing. NS and AEC3 operate at 16kHz only. For 32kHz/48kHz input, audio must be split into 2-3 bands, the lower band processed, upper bands optionally attenuated, then merged. Lives in its own convenience package (not buried in dsp) so the standalone-stage user has a clean path to multi-rate support.

## SIMD backend
Runtime-selectable SIMD acceleration for non-FFT operations (sinc resampler, potentially adaptive filter). Lives in the simd package, separate from FFT backends. FFT backends handle their own SIMD internally.

## FFT backend
The FFT implementation is pluggable via interface. The library ships a pure Go default (Ooura) and may offer CGO-accelerated alternatives (PFFFT, etc.). The consumer chooses at construction time. The interface uses packed in-place format (Forward/Inverse); split re/im format used by AEC3 is a helper on top. See [ADR-0001](docs/adr/0001-pluggable-fft-backend.md).

## Normalized float
Sample representation common in external audio APIs (ffmpeg, PortAudio): float32 values in the range [-1.0, 1.0]. The top-level convenience wrapper offers Normalized variants that handle conversion automatically.

## VADAnalyzer
Interface for pluggable voice activity detection. Implementations are injected into AGC2 via a variadic parameter. Four shipped implementations cover the spectrum from near-zero CPU to neural-network accuracy:
- RMS-based (default) — threshold + hangover, no allocations, no FFT dependency.
- RNN VAD — pure Go GRU network, ~0.5ms/frame, FFT-backed feature extraction.
- GMM VAD — 6-band filterbank with adaptive noise model, no FFT dependency.
- RNNoise VADAdapter — CGO, 48kHz, VAD probability as a byproduct of neural denoising.

## RNN VAD
Pure Go port of WebRTC's RNN-based Voice Activity Detector. Uses a 3-layer neural network (FC → GRU → FC) with pre-trained int8 weights dequantized at init(). Feature extraction pipeline: HPF → pitch buffer → LPC → LP residual → pitch estimation (FFT-based autocorrelation at 12kHz, refinement at 24kHz) → spectral features (480-point FFT via fft.Factory, Opus band energies, cepstral coefficients, DCT). The RNN computes a single VAD probability per 10ms frame. Operates at 24kHz; the RNNVADWrapper handles resampling from other sample rates. Accepts optional fft.Factory for backend selection (default: Bluestein chirp-z).

## GMM VAD
Gaussian Mixture Model VAD with cascaded polyphase all-pass filterbank (6 sub-bands). Uses pre-trained speech/noise GMM parameters from WebRTC common_audio/vad. Noise model means adapt via EMA during non-speech frames. Lighter than RNN VAD, no FFT dependency.

## RNNoise
CGO wrapper around the RNNoise library (Xiph/Mozilla). Alternative noise suppression implementation for users who accept a CGO dependency. Operates at 48kHz on 480-sample frames. Not a replacement for NS and not intended to run alongside it — a separate choice with a different quality/performance tradeoff (neural network vs Wiener filter). Also provides VAD probability; the VADAdapter resamples 16kHz frames to 48kHz for use with agc2's VADAnalyzer interface.
