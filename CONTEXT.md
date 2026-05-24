# Ubiquitous Language

## Diver
Audio pipeline that receives an audio stream, processes it, and outputs a processed stream. Diver-driver is one component of this pipeline.

## Aeol
Permanent name for the library. Short for Aeolus (god of wind). Will live at its own repository as an independent Go module. Replaces working names "Diver-driver" and "sonora". Inspired by WebRTC M145 algorithms, not bit-exact compatible. Quality target: subjectively no worse than the Rust reference on the same recordings.

This is a **toolkit of independent audio processing stages**, not a pipeline. Each stage (NS, AEC3, AGC2, HPF) is importable and usable on its own via subpackages. How to combine and order stages is the caller's responsibility. The top-level AudioProcessing struct in the root package is a convenience wrapper for the common case. All processing stages live in subpackages (ns/, aec3/, agc2/); HighPassFilter should be moved out of the root package into its own subpackage for consistency.

## Capture
Audio coming from the microphone (near-end signal). The signal to be cleaned up.

## Render
Audio coming from the far end (speaker playback). Used by AEC3 as a reference to identify and remove echo.

## FloatS16
Sample representation used by all processing stages: float32 values in the range [-32768, 32767]. Matches WebRTC's "FloatS16" convention. Individual stages (ns, aec3, agc2) accept and return FloatS16 exclusively. Format conversion is the caller's responsibility; utility functions are provided in the root package.

## Statistics
AudioProcessingStats mirrors the WebRTC stats struct for API compatibility. Not all fields are populated — unused fields remain nil. Individual stages also export their own metrics directly (ERLE, GainDb, SpeechProbability, etc.).

## Thread safety
Individual processing stages are not safe for concurrent use. They are stateful library primitives, not services. Synchronization is the caller's responsibility. The top-level AudioProcessing wrapper provides a mutex as convenience.

## Band splitting
Frequency band splitting for multi-rate processing. NS and AEC3 operate at 16kHz only. For 32kHz/48kHz input, audio must be split into 2-3 bands, the lower band processed, upper bands optionally attenuated, then merged. Lives in its own convenience package (not buried in dsp) so the standalone-stage user has a clean path to multi-rate support.

## SIMD backend
Runtime-selectable SIMD acceleration for non-FFT operations (sinc resampler, potentially adaptive filter). Lives in the simd package, separate from FFT backends. FFT backends handle their own SIMD internally.

## FFT backend
The FFT implementation is pluggable via interface. The library ships a pure Go default (Ooura) and may offer CGO-accelerated alternatives (PFFFT, etc.). The consumer chooses at construction time. The interface uses packed in-place format (Forward/Inverse); split re/im format used by AEC3 is a helper on top. See [ADR-0001](docs/adr/0001-pluggable-fft-backend.md).

## Normalized float
Sample representation common in external audio APIs (ffmpeg, PortAudio): float32 values in the range [-1.0, 1.0]. The top-level convenience wrapper offers Normalized variants that handle conversion automatically.
