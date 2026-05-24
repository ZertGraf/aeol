package sonora

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"sonora/aec3"
	agc "sonora/agc2"
	"sonora/dsp"
	"sonora/ns"
)

const (
	warmupFrames    = 50
	measuredFrames  = 2000
	batchSize       = 100
)

type stageTiming struct {
	name    string
	samples []time.Duration
}

func (s *stageTiming) record(total time.Duration, n int) {
	s.samples = append(s.samples, total/time.Duration(n))
}

func (s *stageTiming) sorted() []time.Duration {
	out := make([]time.Duration, len(s.samples))
	copy(out, s.samples)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *stageTiming) stats() (avg, p50, p95, p99, min, max time.Duration) {
	n := len(s.samples)
	if n == 0 {
		return
	}
	d := s.sorted()
	min, max = d[0], d[n-1]
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	avg = sum / time.Duration(n)
	p50 = d[n/2]
	idx95 := n * 95 / 100
	if idx95 >= n {
		idx95 = n - 1
	}
	p95 = d[idx95]
	idx99 := n * 99 / 100
	if idx99 >= n {
		idx99 = n - 1
	}
	p99 = d[idx99]
	return
}

func (s *stageTiming) report(t *testing.T, budget time.Duration) {
	avg, p50, p95, p99, mn, mx := s.stats()
	pct := float64(avg) / float64(budget) * 100
	t.Logf("  %-30s  avg=%-12s p50=%-12s p95=%-12s p99=%-12s min=%-12s max=%-12s [%.2f%% RT]",
		s.name, avg, p50, p95, p99, mn, mx, pct)
}

func sineFrame(size int, freq, sr float64, amp float32) []float32 {
	f := make([]float32, size)
	for i := range f {
		f[i] = amp * float32(math.Sin(2*math.Pi*freq*float64(i)/sr))
	}
	return f
}

func noiseFrame(size int, amp float32) []float32 {
	f := make([]float32, size)
	for i := range f {
		f[i] = amp * (rand.Float32()*2 - 1)
	}
	return f
}

func echoFrame(render []float32, gain, noise float32) []float32 {
	f := make([]float32, len(render))
	for i := range f {
		f[i] = render[i]*gain + noise*(rand.Float32()*2-1)
	}
	return f
}

// --------------------------------------------------------------------
// batch-timed e2e test: measures N frames in one time.Now() span,
// reports per-frame average — immune to windows timer granularity
// --------------------------------------------------------------------

func TestE2E_StageTiming_16kHz(t *testing.T) { runStageTiming(t, 16000) }
func TestE2E_StageTiming_48kHz(t *testing.T) { runStageTiming(t, 48000) }

func runStageTiming(t *testing.T, sampleRate uint32) {
	frameSize := int(sampleRate) / 100
	budget := 10 * time.Millisecond
	batches := measuredFrames / batchSize

	t.Logf("=== E2E Stage Timing @ %d Hz (frame=%d samples, budget=%v) ===",
		sampleRate, frameSize, budget)
	t.Logf("    warmup=%d, measured=%d frames (%d batches x %d)",
		warmupFrames, measuredFrames, batches, batchSize)
	t.Log("")

	preAmpGain := float32(1.5)
	hpf := NewHighPassFilter(sampleRate, 1)
	nsSup := ns.NewSuppressor(ns.Config{Level: ns.SuppressionLevel(NsLevelHigh)})
	gc2 := agc.NewGainController2(agc.Config{
		Enabled: true,
		AdaptiveDigital: agc.AdaptiveDigitalConfig{
			Enabled:                  true,
			HeadroomDb:               1.0,
			MaxGainDb:                30.0,
			InitialGainDb:            8.0,
			MaxGainChangeDbPerSecond: 3.0,
			MaxOutputNoiseLevelDbfs:  -50.0,
		},
		FixedDigital: agc.FixedDigitalConfig{GainDb: 0.0},
	})
	ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), sampleRate, 1)
	capBuf := NewAudioBuffer(StreamConfig{SampleRateHz: sampleRate, NumChannels: 1})
	hasBands := capBuf.Bands() > 1

	total := warmupFrames + measuredFrames
	renders := make([][]float32, total)
	captures := make([][]float32, total)
	for i := range renders {
		renders[i] = sineFrame(frameSize, 300+float64(i%10)*50, float64(sampleRate), 0.3)
		captures[i] = echoFrame(renders[i], 0.4, 0.02)
	}

	// warmup isolated
	for i := 0; i < warmupFrames; i++ {
		r := make([]float32, frameSize)
		copy(r, renders[i])
		c := make([]float32, frameSize)
		copy(c, captures[i])
		for j := range c {
			c[j] *= preAmpGain
		}
		hpf.Process([][]float32{c})
		capBuf.CopyFromFloat([][]float32{c})
		if hasBands {
			capBuf.SplitIntoFrequencyBands()
		}
		lb := capBuf.SplitChannel(0, 0)
		for s := 0; s+aec3.BlockSize <= len(r); s += aec3.BlockSize {
			ec.ProcessRender(r[s : s+aec3.BlockSize])
		}
		for s := 0; s+aec3.BlockSize <= len(lb); s += aec3.BlockSize {
			ec.ProcessCapture(lb[s : s+aec3.BlockSize])
		}
		nsSup.Process(lb)
		if hasBands {
			capBuf.MergeFrequencyBands()
		}
		capBuf.CopyToFloat([][]float32{c})
		gc2.Process(c)
	}

	tPreAmp := &stageTiming{name: "pre-amplifier"}
	tHPF := &stageTiming{name: "high-pass-filter"}
	tBufIn := &stageTiming{name: "buffer-copy-in"}
	tSplit := &stageTiming{name: "band-split"}
	tAECR := &stageTiming{name: "aec3-render"}
	tAECC := &stageTiming{name: "aec3-capture"}
	tNS := &stageTiming{name: "noise-suppression"}
	tMerge := &stageTiming{name: "band-merge"}
	tBufOut := &stageTiming{name: "buffer-copy-out"}
	tAGC := &stageTiming{name: "agc2"}

	for b := 0; b < batches; b++ {
		base := warmupFrames + b*batchSize

		cs := make([][]float32, batchSize)
		rs := make([][]float32, batchSize)
		for j := 0; j < batchSize; j++ {
			cs[j] = make([]float32, frameSize)
			copy(cs[j], captures[base+j])
			rs[j] = make([]float32, frameSize)
			copy(rs[j], renders[base+j])
		}

		// pre-amplifier
		start := time.Now()
		for j := 0; j < batchSize; j++ {
			for k := range cs[j] {
				cs[j][k] *= preAmpGain
			}
		}
		tPreAmp.record(time.Since(start), batchSize)

		// high-pass filter
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			hpf.Process([][]float32{cs[j]})
		}
		tHPF.record(time.Since(start), batchSize)

		// buffer copy in
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			capBuf.CopyFromFloat([][]float32{cs[j]})
		}
		tBufIn.record(time.Since(start), batchSize)

		// band split
		if hasBands {
			start = time.Now()
			for j := 0; j < batchSize; j++ {
				capBuf.CopyFromFloat([][]float32{cs[j]})
				capBuf.SplitIntoFrequencyBands()
			}
			tSplit.record(time.Since(start), batchSize)
		}

		// aec3 render
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			for s := 0; s+aec3.BlockSize <= frameSize; s += aec3.BlockSize {
				ec.ProcessRender(rs[j][s : s+aec3.BlockSize])
			}
		}
		tAECR.record(time.Since(start), batchSize)

		// aec3 capture
		capBuf.CopyFromFloat([][]float32{cs[0]})
		if hasBands {
			capBuf.SplitIntoFrequencyBands()
		}
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			lb := capBuf.SplitChannel(0, 0)
			copy(lb, cs[j][:len(lb)])
			for s := 0; s+aec3.BlockSize <= len(lb); s += aec3.BlockSize {
				ec.ProcessCapture(lb[s : s+aec3.BlockSize])
			}
		}
		tAECC.record(time.Since(start), batchSize)

		// noise suppression
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			lb := capBuf.SplitChannel(0, 0)
			copy(lb, cs[j][:len(lb)])
			nsSup.Process(lb)
		}
		tNS.record(time.Since(start), batchSize)

		// band merge
		if hasBands {
			start = time.Now()
			for j := 0; j < batchSize; j++ {
				capBuf.MergeFrequencyBands()
			}
			tMerge.record(time.Since(start), batchSize)
		}

		// buffer copy out
		out := make([]float32, frameSize)
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			capBuf.CopyToFloat([][]float32{out})
		}
		tBufOut.record(time.Since(start), batchSize)

		// agc2
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			copy(out, cs[j])
			gc2.Process(out)
		}
		tAGC.record(time.Since(start), batchSize)
	}

	t.Log("--- Individual Stages (per-frame, batched measurement) ---")
	stages := []*stageTiming{tPreAmp, tHPF, tBufIn, tSplit, tAECR, tAECC, tNS, tMerge, tBufOut, tAGC}
	for _, st := range stages {
		if len(st.samples) > 0 {
			st.report(t, budget)
		}
	}

	// full pipeline
	t.Log("")
	t.Log("--- Full Pipeline ---")

	apFull, err := NewBuilder().
		SampleRate(sampleRate).Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnablePreAmplifier(PreAmplifierConfig{Gain: preAmpGain}).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer apFull.Close()

	for i := 0; i < warmupFrames; i++ {
		r := make([]float32, frameSize)
		copy(r, renders[i])
		apFull.ProcessRenderFloatNormalized([][]float32{r})
		c := make([]float32, frameSize)
		copy(c, captures[i])
		apFull.ProcessCaptureFloatNormalized([][]float32{c})
	}

	tRender := &stageTiming{name: "FULL-RENDER"}
	tCapture := &stageTiming{name: "FULL-CAPTURE"}
	tPipeline := &stageTiming{name: "FULL-PIPELINE (render+capture)"}

	for b := 0; b < batches; b++ {
		base := warmupFrames + b*batchSize

		rs := make([][]float32, batchSize)
		cs := make([][]float32, batchSize)
		for j := 0; j < batchSize; j++ {
			rs[j] = make([]float32, frameSize)
			copy(rs[j], renders[base+j])
			cs[j] = make([]float32, frameSize)
			copy(cs[j], captures[base+j])
		}

		// render only
		start := time.Now()
		for j := 0; j < batchSize; j++ {
			apFull.ProcessRenderFloatNormalized([][]float32{rs[j]})
		}
		tRender.record(time.Since(start), batchSize)

		// capture only
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			apFull.ProcessCaptureFloatNormalized([][]float32{cs[j]})
		}
		tCapture.record(time.Since(start), batchSize)

		// full round-trip (re-copy)
		for j := 0; j < batchSize; j++ {
			copy(rs[j], renders[base+j])
			copy(cs[j], captures[base+j])
		}
		start = time.Now()
		for j := 0; j < batchSize; j++ {
			apFull.ProcessRenderFloatNormalized([][]float32{rs[j]})
			apFull.ProcessCaptureFloatNormalized([][]float32{cs[j]})
		}
		tPipeline.record(time.Since(start), batchSize)
	}

	tRender.report(t, budget)
	tCapture.report(t, budget)
	tPipeline.report(t, budget)

	avg, _, p95, _, _, _ := tPipeline.stats()
	t.Log("")
	t.Logf("  Real-time budget:  %v", budget)
	t.Logf("  Avg pipeline:      %v (%.2f%% of budget)", avg, float64(avg)/float64(budget)*100)
	t.Logf("  P95 pipeline:      %v (%.2f%% of budget)", p95, float64(p95)/float64(budget)*100)

	if avg > budget {
		t.Errorf("FAIL: average latency %v exceeds real-time budget %v", avg, budget)
	}
}

// --------------------------------------------------------------------
// additional e2e scenarios
// --------------------------------------------------------------------

func TestE2E_MultiChannel(t *testing.T) {
	sampleRate := uint32(48000)
	channels := uint16(2)
	frameSize := int(sampleRate) / 100
	budget := 10 * time.Millisecond
	batches := measuredFrames / batchSize

	t.Logf("=== Multi-Channel @ %dHz x %dch ===", sampleRate, channels)

	ap, err := NewBuilder().
		SampleRate(sampleRate).Channels(channels).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()

	// warmup
	for i := 0; i < warmupFrames; i++ {
		r := make([][]float32, channels)
		c := make([][]float32, channels)
		for ch := range r {
			r[ch] = sineFrame(frameSize, 300+float64(ch)*200, float64(sampleRate), 0.3)
			c[ch] = echoFrame(r[ch], 0.4, 0.02)
		}
		ap.ProcessRenderFloatNormalized(r)
		ap.ProcessCaptureFloatNormalized(c)
	}

	tR := &stageTiming{name: "render-2ch"}
	tC := &stageTiming{name: "capture-2ch"}

	for b := 0; b < batches; b++ {
		rBatch := make([][][]float32, batchSize)
		cBatch := make([][][]float32, batchSize)
		for j := 0; j < batchSize; j++ {
			r := make([][]float32, channels)
			c := make([][]float32, channels)
			for ch := range r {
				r[ch] = sineFrame(frameSize, 300+float64(ch)*200, float64(sampleRate), 0.3)
				c[ch] = echoFrame(r[ch], 0.4, 0.02)
			}
			rBatch[j] = r
			cBatch[j] = c
		}

		start := time.Now()
		for j := 0; j < batchSize; j++ {
			ap.ProcessRenderFloatNormalized(rBatch[j])
		}
		tR.record(time.Since(start), batchSize)

		start = time.Now()
		for j := 0; j < batchSize; j++ {
			ap.ProcessCaptureFloatNormalized(cBatch[j])
		}
		tC.record(time.Since(start), batchSize)
	}

	tR.report(t, budget)
	tC.report(t, budget)
}

func TestE2E_BandSplitting(t *testing.T) {
	t.Log("=== Band Splitting/Merging Overhead ===")

	rates := []uint32{16000, 32000, 48000}
	for _, rate := range rates {
		numBands := numBandsForRate(rate)
		if numBands == 1 {
			t.Logf("  %dkHz: single band, no splitting", rate/1000)
			continue
		}

		frameSize := int(rate) / 100
		buf := NewAudioBuffer(StreamConfig{SampleRateHz: rate, NumChannels: 1})
		frame := noiseFrame(frameSize, 0.3)

		for i := 0; i < warmupFrames; i++ {
			buf.CopyFromFloat([][]float32{frame})
			buf.SplitIntoFrequencyBands()
			buf.MergeFrequencyBands()
		}

		tS := &stageTiming{name: fmt.Sprintf("split-%dkHz-%dband", rate/1000, numBands)}
		tM := &stageTiming{name: fmt.Sprintf("merge-%dkHz-%dband", rate/1000, numBands)}

		batches := measuredFrames / batchSize
		for b := 0; b < batches; b++ {
			start := time.Now()
			for j := 0; j < batchSize; j++ {
				buf.CopyFromFloat([][]float32{frame})
				buf.SplitIntoFrequencyBands()
			}
			tS.record(time.Since(start), batchSize)

			start = time.Now()
			for j := 0; j < batchSize; j++ {
				buf.MergeFrequencyBands()
			}
			tM.record(time.Since(start), batchSize)
		}

		tS.report(t, 10*time.Millisecond)
		tM.report(t, 10*time.Millisecond)
	}
}

func TestE2E_AEC3Convergence(t *testing.T) {
	sampleRate := uint32(16000)
	frameSize := int(sampleRate) / 100
	budget := 10 * time.Millisecond
	totalFrames := 500
	batches := totalFrames / batchSize

	t.Log("=== AEC3 Convergence Timing ===")

	ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), sampleRate, 1)

	tR := &stageTiming{name: "aec3-render-converging"}
	tC := &stageTiming{name: "aec3-capture-converging"}

	for b := 0; b < batches; b++ {
		rs := make([][]float32, batchSize)
		cs := make([][]float32, batchSize)
		for j := 0; j < batchSize; j++ {
			rs[j] = sineFrame(frameSize, 440, float64(sampleRate), 0.5)
			cs[j] = echoFrame(rs[j], 0.6, 0.01)
		}

		start := time.Now()
		for j := 0; j < batchSize; j++ {
			for s := 0; s+aec3.BlockSize <= frameSize; s += aec3.BlockSize {
				ec.ProcessRender(rs[j][s : s+aec3.BlockSize])
			}
		}
		tR.record(time.Since(start), batchSize)

		start = time.Now()
		for j := 0; j < batchSize; j++ {
			for s := 0; s+aec3.BlockSize <= frameSize; s += aec3.BlockSize {
				ec.ProcessCapture(cs[j][s : s+aec3.BlockSize])
			}
		}
		tC.record(time.Since(start), batchSize)
	}

	tR.report(t, budget)
	tC.report(t, budget)
	t.Logf("  After %d frames: ERLE=%.2f dB, Delay=%d ms", totalFrames, ec.ERLE(), ec.Delay())
}

func TestE2E_NSLevels(t *testing.T) {
	sampleRate := uint32(16000)
	frameSize := int(sampleRate) / 100
	budget := 10 * time.Millisecond
	batches := measuredFrames / batchSize

	t.Log("=== Noise Suppression by Level ===")

	levels := []struct {
		name  string
		level ns.SuppressionLevel
	}{
		{"ns-low", ns.SuppressionLevel(NsLevelLow)},
		{"ns-moderate", ns.SuppressionLevel(NsLevelModerate)},
		{"ns-high", ns.SuppressionLevel(NsLevelHigh)},
		{"ns-very-high", ns.SuppressionLevel(NsLevelVeryHigh)},
	}

	for _, lvl := range levels {
		sup := ns.NewSuppressor(ns.Config{Level: lvl.level})
		for i := 0; i < warmupFrames; i++ {
			sup.Process(noiseFrame(frameSize, 0.05))
		}

		st := &stageTiming{name: lvl.name}
		for b := 0; b < batches; b++ {
			frames := make([][]float32, batchSize)
			for j := range frames {
				frames[j] = noiseFrame(frameSize, 0.05)
			}
			start := time.Now()
			for j := 0; j < batchSize; j++ {
				sup.Process(frames[j])
			}
			st.record(time.Since(start), batchSize)
		}
		st.report(t, budget)
	}
}

func TestE2E_Int16VsFloat32(t *testing.T) {
	sampleRate := uint32(16000)
	frameSize := int(sampleRate) / 100
	budget := 10 * time.Millisecond
	batches := measuredFrames / batchSize

	t.Log("=== Int16 vs Float32 Path ===")

	apF, _ := NewBuilder().SampleRate(sampleRate).Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	defer apF.Close()

	apI, _ := NewBuilder().SampleRate(sampleRate).Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	defer apI.Close()

	for i := 0; i < warmupFrames; i++ {
		f := sineFrame(frameSize, 440, float64(sampleRate), 0.3)
		fCopy := make([]float32, frameSize)
		copy(fCopy, f)
		apF.ProcessCaptureFloatNormalized([][]float32{fCopy})
		buf := make([]int16, frameSize)
		for j := range buf {
			buf[j] = int16(f[j] * math.MaxInt16)
		}
		apI.ProcessCaptureInt16(buf)
	}

	tF := &stageTiming{name: "capture-float32"}
	tI := &stageTiming{name: "capture-int16"}

	for b := 0; b < batches; b++ {
		fFrames := make([][]float32, batchSize)
		iFrames := make([][]int16, batchSize)
		for j := 0; j < batchSize; j++ {
			fFrames[j] = sineFrame(frameSize, 440, float64(sampleRate), 0.3)
			iFrames[j] = make([]int16, frameSize)
			for k := range iFrames[j] {
				iFrames[j][k] = int16(fFrames[j][k] * math.MaxInt16)
			}
		}

		start := time.Now()
		for j := 0; j < batchSize; j++ {
			apF.ProcessCaptureFloatNormalized([][]float32{fFrames[j]})
		}
		tF.record(time.Since(start), batchSize)

		start = time.Now()
		for j := 0; j < batchSize; j++ {
			apI.ProcessCaptureInt16(iFrames[j])
		}
		tI.record(time.Since(start), batchSize)
	}

	tF.report(t, budget)
	tI.report(t, budget)

	avgF, _, _, _, _, _ := tF.stats()
	avgI, _, _, _, _, _ := tI.stats()
	if avgF > 0 {
		t.Logf("  Int16 overhead vs Float32: %.1f%%", float64(avgI-avgF)/float64(avgF)*100)
	}
}

func TestE2E_Throughput(t *testing.T) {
	configs := []struct {
		name       string
		sampleRate uint32
		channels   uint16
		aec, ns, agc bool
	}{
		{"minimal-16k", 16000, 1, false, false, false},
		{"ns-only-16k", 16000, 1, false, true, false},
		{"agc-only-16k", 16000, 1, false, false, true},
		{"aec-only-16k", 16000, 1, true, false, false},
		{"full-16k-1ch", 16000, 1, true, true, true},
		{"full-48k-1ch", 48000, 1, true, true, true},
		{"full-48k-2ch", 48000, 2, true, true, true},
	}

	t.Log("=== Throughput (frames/sec, realtime multiplier) ===")
	t.Log("")

	for _, cfg := range configs {
		b := NewBuilder().SampleRate(cfg.sampleRate).Channels(cfg.channels).
			EnableHighPassFilter(DefaultHighPassFilterConfig())
		if cfg.aec {
			b = b.EnableEchoCanceller(DefaultEchoCancellerConfig())
		}
		if cfg.ns {
			b = b.EnableNoiseSuppression(NsConfig{Level: NsLevelHigh})
		}
		if cfg.agc {
			b = b.EnableGainController2(DefaultGainController2Config())
		}

		ap, err := b.Build()
		if err != nil {
			t.Fatalf("build %s: %v", cfg.name, err)
		}

		fs := int(cfg.sampleRate) / 100
		nch := int(cfg.channels)

		for i := 0; i < warmupFrames; i++ {
			r := make([][]float32, nch)
			c := make([][]float32, nch)
			for ch := 0; ch < nch; ch++ {
				r[ch] = sineFrame(fs, 300, float64(cfg.sampleRate), 0.3)
				c[ch] = echoFrame(r[ch], 0.4, 0.02)
			}
			if cfg.aec {
				ap.ProcessRenderFloatNormalized(r)
			}
			ap.ProcessCaptureFloatNormalized(c)
		}

		n := 3000
		start := time.Now()
		for i := 0; i < n; i++ {
			r := make([][]float32, nch)
			c := make([][]float32, nch)
			for ch := 0; ch < nch; ch++ {
				r[ch] = sineFrame(fs, 300+float64(i%5)*100, float64(cfg.sampleRate), 0.3)
				c[ch] = echoFrame(r[ch], 0.4, 0.02)
			}
			if cfg.aec {
				ap.ProcessRenderFloatNormalized(r)
			}
			ap.ProcessCaptureFloatNormalized(c)
		}
		elapsed := time.Since(start)
		ap.Close()

		fps := float64(n) / elapsed.Seconds()
		rtx := fps / 100.0
		t.Logf("  %-18s  %8.0f frames/s  %6.1fx realtime  avg=%v/frame",
			cfg.name, fps, rtx, elapsed/time.Duration(n))
	}
}

func TestE2E_SplittingFilter(t *testing.T) {
	t.Log("=== Splitting Filter (analysis+synthesis round-trip) ===")

	for _, rate := range []uint32{32000, 48000} {
		numBands := numBandsForRate(rate)
		sf := dsp.NewSplittingFilter(1, numBands)
		frameSize := int(rate) / 100
		fpb := frameSize / numBands

		input := [][]float32{noiseFrame(frameSize, 0.5)}
		output := make([][][]float32, 1)
		output[0] = make([][]float32, numBands)
		for b := 0; b < numBands; b++ {
			output[0][b] = make([]float32, fpb)
		}

		for i := 0; i < warmupFrames; i++ {
			sf.Analysis(input, output)
			sf.Synthesis(output, input)
		}

		st := &stageTiming{name: fmt.Sprintf("split+merge-%dkHz", rate/1000)}
		batches := measuredFrames / batchSize
		for b := 0; b < batches; b++ {
			start := time.Now()
			for j := 0; j < batchSize; j++ {
				sf.Analysis(input, output)
				sf.Synthesis(output, input)
			}
			st.record(time.Since(start), batchSize)
		}
		st.report(t, 10*time.Millisecond)
	}
}

// --------------------------------------------------------------------
// go test -bench=. benchmarks — precise via testing.B
// --------------------------------------------------------------------

func BenchmarkStage_PreAmplifier(b *testing.B) {
	frame := sineFrame(160, 440, 16000, 0.3)
	gain := float32(1.5)
	b.ResetTimer()
	for range b.N {
		for i := range frame {
			frame[i] *= gain
		}
	}
}

func BenchmarkStage_HighPassFilter_16k(b *testing.B) {
	hpf := NewHighPassFilter(16000, 1)
	frame := sineFrame(160, 440, 16000, 0.3)
	data := [][]float32{frame}
	b.ResetTimer()
	for range b.N {
		hpf.Process(data)
	}
}

func BenchmarkStage_HighPassFilter_48k(b *testing.B) {
	hpf := NewHighPassFilter(48000, 1)
	frame := sineFrame(480, 440, 48000, 0.3)
	data := [][]float32{frame}
	b.ResetTimer()
	for range b.N {
		hpf.Process(data)
	}
}

func BenchmarkStage_BandSplit_48k(b *testing.B) {
	buf := NewAudioBuffer(StreamConfig{SampleRateHz: 48000, NumChannels: 1})
	frame := noiseFrame(480, 0.3)
	b.ResetTimer()
	for range b.N {
		buf.CopyFromFloat([][]float32{frame})
		buf.SplitIntoFrequencyBands()
	}
}

func BenchmarkStage_BandMerge_48k(b *testing.B) {
	buf := NewAudioBuffer(StreamConfig{SampleRateHz: 48000, NumChannels: 1})
	frame := noiseFrame(480, 0.3)
	buf.CopyFromFloat([][]float32{frame})
	buf.SplitIntoFrequencyBands()
	b.ResetTimer()
	for range b.N {
		buf.MergeFrequencyBands()
	}
}

func BenchmarkStage_AEC3Render_16k(b *testing.B) {
	ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), 16000, 1)
	frame := sineFrame(160, 300, 16000, 0.3)
	for i := 0; i < warmupFrames; i++ {
		for s := 0; s+aec3.BlockSize <= 160; s += aec3.BlockSize {
			ec.ProcessRender(frame[s : s+aec3.BlockSize])
		}
	}
	b.ResetTimer()
	for range b.N {
		for s := 0; s+aec3.BlockSize <= 160; s += aec3.BlockSize {
			ec.ProcessRender(frame[s : s+aec3.BlockSize])
		}
	}
}

func BenchmarkStage_AEC3Capture_16k(b *testing.B) {
	ec := aec3.NewEchoCanceller3(aec3.DefaultConfig(), 16000, 1)
	render := sineFrame(160, 300, 16000, 0.3)
	capture := echoFrame(render, 0.5, 0.01)
	for i := 0; i < warmupFrames; i++ {
		for s := 0; s+aec3.BlockSize <= 160; s += aec3.BlockSize {
			ec.ProcessRender(render[s : s+aec3.BlockSize])
		}
		for s := 0; s+aec3.BlockSize <= 160; s += aec3.BlockSize {
			ec.ProcessCapture(capture[s : s+aec3.BlockSize])
		}
	}
	b.ResetTimer()
	for range b.N {
		for s := 0; s+aec3.BlockSize <= 160; s += aec3.BlockSize {
			ec.ProcessRender(render[s : s+aec3.BlockSize])
			ec.ProcessCapture(capture[s : s+aec3.BlockSize])
		}
	}
}

func BenchmarkStage_NoiseSuppression_16k(b *testing.B) {
	sup := ns.NewSuppressor(ns.Config{Level: ns.SuppressionLevel(NsLevelHigh)})
	frame := noiseFrame(160, 0.05)
	for i := 0; i < warmupFrames; i++ {
		sup.Process(frame)
	}
	b.ResetTimer()
	for range b.N {
		sup.Process(frame)
	}
}

func BenchmarkStage_AGC2_16k(b *testing.B) {
	gc2 := agc.NewGainController2(agc.Config{
		Enabled: true,
		AdaptiveDigital: agc.AdaptiveDigitalConfig{
			Enabled:                  true,
			HeadroomDb:               1.0,
			MaxGainDb:                30.0,
			InitialGainDb:            8.0,
			MaxGainChangeDbPerSecond: 3.0,
			MaxOutputNoiseLevelDbfs:  -50.0,
		},
		FixedDigital: agc.FixedDigitalConfig{GainDb: 0.0},
	})
	frame := sineFrame(160, 440, 16000, 0.01)
	for i := 0; i < warmupFrames; i++ {
		gc2.Process(frame)
	}
	b.ResetTimer()
	for range b.N {
		gc2.Process(frame)
	}
}

func BenchmarkPipeline_Full_16k(b *testing.B) {
	ap, _ := NewBuilder().SampleRate(16000).Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	defer ap.Close()

	render := sineFrame(160, 300, 16000, 0.3)
	capture := echoFrame(render, 0.4, 0.02)
	rData := [][]float32{render}
	cData := [][]float32{capture}

	for i := 0; i < warmupFrames; i++ {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
	b.ResetTimer()
	for range b.N {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
}

func BenchmarkPipeline_Full_48k(b *testing.B) {
	ap, _ := NewBuilder().SampleRate(48000).Channels(1).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	defer ap.Close()

	render := sineFrame(480, 300, 48000, 0.3)
	capture := echoFrame(render, 0.4, 0.02)
	rData := [][]float32{render}
	cData := [][]float32{capture}

	for i := 0; i < warmupFrames; i++ {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
	b.ResetTimer()
	for range b.N {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
}

func BenchmarkPipeline_Full_48k_2ch(b *testing.B) {
	ap, _ := NewBuilder().SampleRate(48000).Channels(2).
		EnableHighPassFilter(DefaultHighPassFilterConfig()).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		EnableGainController2(DefaultGainController2Config()).
		Build()
	defer ap.Close()

	r0 := sineFrame(480, 300, 48000, 0.3)
	r1 := sineFrame(480, 500, 48000, 0.3)
	c0 := echoFrame(r0, 0.4, 0.02)
	c1 := echoFrame(r1, 0.4, 0.02)
	rData := [][]float32{r0, r1}
	cData := [][]float32{c0, c1}

	for i := 0; i < warmupFrames; i++ {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
	b.ResetTimer()
	for range b.N {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
}

func BenchmarkPipeline_NSOnly_16k(b *testing.B) {
	ap, _ := NewBuilder().SampleRate(16000).Channels(1).
		EnableNoiseSuppression(NsConfig{Level: NsLevelHigh}).
		Build()
	defer ap.Close()

	frame := noiseFrame(160, 0.05)
	data := [][]float32{frame}
	for i := 0; i < warmupFrames; i++ {
		ap.ProcessCaptureFloatNormalized(data)
	}
	b.ResetTimer()
	for range b.N {
		ap.ProcessCaptureFloatNormalized(data)
	}
}

func BenchmarkPipeline_AECOnly_16k(b *testing.B) {
	ap, _ := NewBuilder().SampleRate(16000).Channels(1).
		EnableEchoCanceller(DefaultEchoCancellerConfig()).
		Build()
	defer ap.Close()

	render := sineFrame(160, 300, 16000, 0.3)
	capture := echoFrame(render, 0.5, 0.01)
	rData := [][]float32{render}
	cData := [][]float32{capture}

	for i := 0; i < warmupFrames; i++ {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
	b.ResetTimer()
	for range b.N {
		ap.ProcessRenderFloatNormalized(rData)
		ap.ProcessCaptureFloatNormalized(cData)
	}
}
