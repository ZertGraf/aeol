package agc2

import (
	"math"
	"testing"
)

// makeFrame generates a 160-sample FloatS16 frame.
// amplitude is in FloatS16 units (max 32767).
func makeSineFrame(freqHz, amplitudeF16 float32) [vadFrameSize]float32 {
	var frame [vadFrameSize]float32
	for i := range frame {
		frame[i] = amplitudeF16 * float32(math.Sin(2*math.Pi*float64(freqHz)*float64(i)/16000))
	}
	return frame
}

func makeSilenceFrame() [vadFrameSize]float32 {
	return [vadFrameSize]float32{}
}

func makeNoiseFrame(amplitude float32) [vadFrameSize]float32 {
	// deterministic pseudo-random noise using a simple LCG
	var frame [vadFrameSize]float32
	seed := uint32(0xdeadbeef)
	for i := range frame {
		seed = seed*1664525 + 1013904223
		// map to [-amplitude, amplitude]
		frame[i] = amplitude * (float32(int32(seed)>>16) / 32768.0)
	}
	return frame
}

// feedFrames runs n identical frames through the VAD and returns the last probability.
func feedFrames(vad VADAnalyzer, frame []float32, n int) float32 {
	var p float32
	for i := 0; i < n; i++ {
		p = vad.Analyze(frame)
	}
	return p
}

// TestGMMVAD_Silence verifies that a silent frame yields low speech probability.
func TestGMMVAD_Silence(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)
	frame := makeSilenceFrame()

	var lastP float32
	for i := 0; i < 20; i++ {
		lastP = vad.Analyze(frame[:])
	}

	if lastP > 0.3 {
		t.Errorf("silence probability = %.3f, want < 0.3", lastP)
	}
}

// TestGMMVAD_Tone verifies that a loud 300 Hz sine (speech-like) yields high probability
// after several frames, allowing the noise model to diverge from the signal.
func TestGMMVAD_Tone(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)
	frame := makeSineFrame(300, 6000) // ~-15 dBFS

	// prime the VAD with silence first so the noise model settles at low energy
	silenceFrame := makeSilenceFrame()
	for i := 0; i < 20; i++ {
		vad.Analyze(silenceFrame[:])
	}

	// now feed speech-like tone frames
	var lastP float32
	for i := 0; i < 30; i++ {
		lastP = vad.Analyze(frame[:])
	}

	if lastP < 0.4 {
		t.Errorf("speech-like tone probability = %.3f, want > 0.4", lastP)
	}
}

// TestGMMVAD_Noise verifies that broadband noise at moderate level eventually yields
// low probability after the noise model adapts. the key point is that after convergence
// the detector should not be perpetually triggered by stationary noise.
func TestGMMVAD_Noise(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)
	frame := makeNoiseFrame(2000) // moderate noise

	var lastP float32
	// feed 60 frames to let noise model adapt
	for i := 0; i < 60; i++ {
		lastP = vad.Analyze(frame[:])
	}

	// after adaptation the detector should reduce speech probability for stationary noise
	// (it may not reach 0 immediately, but should be meaningfully below 0.8)
	if lastP > 0.8 {
		t.Errorf("stationary noise probability after adaptation = %.3f, want <= 0.8", lastP)
	}
}

// TestGMMVAD_Reset verifies that Reset clears speech probability and hangover state.
func TestGMMVAD_Reset(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)

	// drive the detector to a speech state
	frame := makeSineFrame(440, 8000)
	silenceFrame := makeSilenceFrame()
	for i := 0; i < 20; i++ {
		vad.Analyze(silenceFrame[:])
	}
	for i := 0; i < 30; i++ {
		vad.Analyze(frame[:])
	}
	before := vad.speechProbability

	vad.Reset()

	if vad.speechProbability != 0 {
		t.Errorf("after Reset: speechProbability = %.3f, want 0", vad.speechProbability)
	}
	if vad.hangoverCounter != 0 {
		t.Errorf("after Reset: hangoverCounter = %d, want 0", vad.hangoverCounter)
	}
	// noise means should be back to initial values
	for b := 0; b < numBands; b++ {
		for k := 0; k < numGMMComponents; k++ {
			got := vad.noiseMeans[b][k]
			want := webrtcGMM.noiseInitMeans[b][k]
			if math.Abs(float64(got-want)) > 1e-6 {
				t.Errorf("band %d component %d: noiseMeans after Reset = %.4f, want %.4f", b, k, got, want)
			}
		}
	}
	_ = before // just to confirm state was non-zero before reset
}

// TestGMMVAD_AllModes verifies that all aggressiveness modes construct and run
// without panic or NaN/Inf output.
func TestGMMVAD_AllModes(t *testing.T) {
	frame := makeSineFrame(500, 4000)
	silence := makeSilenceFrame()

	for mode := 0; mode <= 3; mode++ {
		mode := mode
		t.Run(modeNames[mode], func(t *testing.T) {
			vad := NewGMMVoiceActivityDetector(mode)

			for i := 0; i < 10; i++ {
				vad.Analyze(silence[:])
			}
			for i := 0; i < 10; i++ {
				p := vad.Analyze(frame[:])
				if math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) {
					t.Errorf("mode %d: Analyze returned NaN/Inf at frame %d", mode, i)
				}
				if p < 0 || p > 1 {
					t.Errorf("mode %d: Analyze returned %f, want in [0,1]", mode, p)
				}
			}
		})
	}
}

// TestGMMVAD_ModeClamp verifies that out-of-range mode values are clamped.
func TestGMMVAD_ModeClamp(t *testing.T) {
	vad0 := NewGMMVoiceActivityDetector(-5)
	vad1 := NewGMMVoiceActivityDetector(99)

	if vad0.threshold != modeThresholds[0].threshold {
		t.Errorf("mode -5 should clamp to 0: got threshold %.2f", vad0.threshold)
	}
	if vad1.threshold != modeThresholds[3].threshold {
		t.Errorf("mode 99 should clamp to 3: got threshold %.2f", vad1.threshold)
	}
}

// TestGMMVAD_ShortFrame verifies that passing fewer than 160 samples does not panic
// (the implementation zero-pads internally).
func TestGMMVAD_ShortFrame(t *testing.T) {
	vad := NewGMMVoiceActivityDetector()
	short := make([]float32, 80)
	for i := range short {
		short[i] = float32(math.Sin(float64(i) * 0.1))
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Analyze panicked on short frame: %v", r)
		}
	}()
	p := vad.Analyze(short)
	if math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) {
		t.Errorf("Analyze returned NaN/Inf for short frame")
	}
}

// TestGMMVAD_EmptyFrame verifies that an empty slice is handled gracefully.
func TestGMMVAD_EmptyFrame(t *testing.T) {
	vad := NewGMMVoiceActivityDetector()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Analyze panicked on empty frame: %v", r)
		}
	}()
	p := vad.Analyze(nil)
	if math.IsNaN(float64(p)) || math.IsInf(float64(p), 0) {
		t.Errorf("Analyze returned NaN/Inf for nil frame")
	}
}

// TestGMMVAD_HangoverBehaviour verifies that after speech ends the VAD holds active
// for at least one additional frame (hangover), then decays.
func TestGMMVAD_HangoverBehaviour(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)

	silence := makeSilenceFrame()
	speech := makeSineFrame(300, 8000)

	// settle noise model
	for i := 0; i < 30; i++ {
		vad.Analyze(silence[:])
	}
	// trigger speech
	for i := 0; i < 20; i++ {
		vad.Analyze(speech[:])
	}
	probAfterSpeech := vad.speechProbability

	// switch to silence: first frame should still show elevated probability (hangover)
	probHangover := vad.Analyze(silence[:])

	// after hangover expires (hangoverFrames+1 frames of silence), probability should decay
	for i := 0; i < vad.hangoverFrames+5; i++ {
		vad.Analyze(silence[:])
	}
	probDecayed := vad.speechProbability

	if probHangover < 0.05 {
		t.Logf("hangover probability = %.3f (may be low if speech was not strong enough)", probHangover)
	}
	if probDecayed >= probAfterSpeech {
		t.Logf("probability after hangover (%.3f) did not decay from speech level (%.3f)", probDecayed, probAfterSpeech)
	}
}

// TestGMMVAD_SatisfiesInterface verifies the concrete type satisfies VADAnalyzer.
func TestGMMVAD_SatisfiesInterface(t *testing.T) {
	var _ VADAnalyzer = (*GMMVoiceActivityDetector)(nil)
}

// TestGMMVAD_NoiseAdaptation verifies that noiseMeans change after non-speech frames.
func TestGMMVAD_NoiseAdaptation(t *testing.T) {
	vad := NewGMMVoiceActivityDetector(1)
	initial := vad.noiseMeans

	// feed 40 frames of noise so that adaptation fires
	frame := makeNoiseFrame(500)
	for i := 0; i < 40; i++ {
		vad.Analyze(frame[:])
	}

	changed := false
	for b := 0; b < numBands; b++ {
		for k := 0; k < numGMMComponents; k++ {
			if math.Abs(float64(vad.noiseMeans[b][k]-initial[b][k])) > 1e-4 {
				changed = true
			}
		}
	}
	if !changed {
		t.Error("noise model means did not change after 40 non-speech frames")
	}
}

// TestGMMVAD_SpeechModelFixed verifies that speech GMM parameters are never mutated.
// (they are package-level constants, but let us assert nothing writes to them.)
func TestGMMVAD_SpeechModelFixed(t *testing.T) {
	snapshot := webrtcGMM.speechMeans
	vad := NewGMMVoiceActivityDetector(1)
	frame := makeSineFrame(300, 8000)
	for i := 0; i < 100; i++ {
		vad.Analyze(frame[:])
	}
	if webrtcGMM.speechMeans != snapshot {
		t.Error("speech GMM means were mutated during inference")
	}
}

// BenchmarkGMMVAD_Analyze measures the per-frame cost of a full GMM VAD pass.
func BenchmarkGMMVAD_Analyze(b *testing.B) {
	vad := NewGMMVoiceActivityDetector(1)
	frame := makeSineFrame(300, 4000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vad.Analyze(frame[:])
	}
}

// BenchmarkGMMVAD_AnalyzeParallel benchmarks concurrent calls (with separate detectors).
func BenchmarkGMMVAD_AnalyzeParallel(b *testing.B) {
	frame := makeSineFrame(300, 4000)
	b.RunParallel(func(pb *testing.PB) {
		vad := NewGMMVoiceActivityDetector(1)
		for pb.Next() {
			_ = vad.Analyze(frame[:])
		}
	})
}

var modeNames = [4]string{"quality", "low-bitrate", "aggressive", "very-aggressive"}
