package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sonora"
)

func main() {
	inDir := flag.String("in", "", "input directory with .wav files")
	outDir := flag.String("out", "", "output directory")
	renderDir := flag.String("render", "", "render directory for AEC3 (optional, enables echo cancellation)")
	nsLevel := flag.Int("ns", 2, "noise suppression level: 0=low, 1=moderate, 2=high, 3=very-high")
	noNs := flag.Bool("no-ns", false, "disable noise suppression")
	noAgc := flag.Bool("no-agc", false, "disable AGC2")
	noHpf := flag.Bool("no-hpf", false, "disable high-pass filter")
	flag.Parse()

	if *inDir == "" || *outDir == "" {
		fmt.Fprintf(os.Stderr, "usage: process_wav -in <input_dir> -out <output_dir> [-render <render_dir>] [-ns 0..3]\n")
		os.Exit(1)
	}

	os.MkdirAll(*outDir, 0o755)

	files, _ := filepath.Glob(filepath.Join(*inDir, "*.wav"))
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no .wav files in %s\n", *inDir)
		os.Exit(1)
	}

	for _, inPath := range files {
		name := filepath.Base(inPath)
		outPath := filepath.Join(*outDir, name)

		var renderPath string
		if *renderDir != "" {
			renderPath = filepath.Join(*renderDir, name)
			if _, err := os.Stat(renderPath); err != nil {
				renderPath = ""
			}
		}

		fmt.Printf("%s -> %s", name, outPath)
		if renderPath != "" {
			fmt.Printf(" (render: %s)", filepath.Base(renderPath))
		}
		fmt.Println()

		if err := processFile(inPath, outPath, renderPath, sonora.NsLevel(*nsLevel), *noNs, *noAgc, *noHpf); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		}
	}
}

func processFile(capturePath, outPath, renderPath string, nsLevel sonora.NsLevel, noNs, noAgc, noHpf bool) error {
	capHdr, capSamples, err := readWav(capturePath)
	if err != nil {
		return fmt.Errorf("read capture: %w", err)
	}

	sampleRate := capHdr.sampleRate
	numChannels := capHdr.numChannels

	if sampleRate != 16000 && sampleRate != 32000 && sampleRate != 48000 {
		return fmt.Errorf("unsupported sample rate %d (need 16000/32000/48000)", sampleRate)
	}

	frameSize := int(sampleRate) / 100 * int(numChannels)

	builder := sonora.NewBuilder().
		SampleRate(sampleRate).
		Channels(numChannels)

	if !noNs {
		builder.EnableNoiseSuppression(sonora.NsConfig{Level: nsLevel})
	}
	if !noHpf {
		builder.EnableHighPassFilter(sonora.DefaultHighPassFilterConfig())
	}
	if !noAgc {
		agcCfg := sonora.DefaultGainController2Config()
		agcCfg.Enabled = true
		agcCfg.AdaptiveDigital.HeadroomDb = 0
		builder.EnableGainController2(agcCfg)
	}

	useAEC := renderPath != ""
	var renderSamples []int16

	if useAEC {
		rHdr, rSamples, err := readWav(renderPath)
		if err != nil {
			return fmt.Errorf("read render: %w", err)
		}
		if rHdr.sampleRate != sampleRate || rHdr.numChannels != numChannels {
			return fmt.Errorf("render format mismatch: capture=%dHz/%dch, render=%dHz/%dch",
				sampleRate, numChannels, rHdr.sampleRate, rHdr.numChannels)
		}
		renderSamples = rSamples
		builder.EnableEchoCanceller(sonora.DefaultEchoCancellerConfig())
	}

	ap, err := builder.Build()
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer ap.Close()

	out := make([]int16, 0, len(capSamples))
	frame := make([]int16, frameSize)
	renderFrame := make([]int16, frameSize)

	totalFrames := len(capSamples) / frameSize
	for i := 0; i < totalFrames; i++ {
		offset := i * frameSize
		copy(frame, capSamples[offset:offset+frameSize])

		if useAEC {
			if offset+frameSize <= len(renderSamples) {
				copy(renderFrame, renderSamples[offset:offset+frameSize])
			} else {
				clear(renderFrame)
			}
			ap.ProcessRenderInt16(renderFrame)
		}

		if err := ap.ProcessCaptureInt16(frame); err != nil {
			return fmt.Errorf("frame %d: %w", i, err)
		}

		out = append(out, frame...)
	}

	tail := len(capSamples) % frameSize
	if tail > 0 {
		out = append(out, capSamples[totalFrames*frameSize:]...)
	}

	if err := writeWav(outPath, capHdr, out); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	durMs := len(capSamples) / int(numChannels) * 1000 / int(sampleRate)
	fmt.Printf("  %d frames, %d.%03ds, %dHz %dch\n", totalFrames, durMs/1000, durMs%1000, sampleRate, numChannels)
	return nil
}

type wavHeader struct {
	sampleRate    uint32
	numChannels   uint16
	bitsPerSample uint16
}

func readWav(path string) (wavHeader, []int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return wavHeader{}, nil, err
	}
	defer f.Close()

	var riffID [4]byte
	var fileSize uint32
	var waveID [4]byte
	binary.Read(f, binary.LittleEndian, &riffID)
	binary.Read(f, binary.LittleEndian, &fileSize)
	binary.Read(f, binary.LittleEndian, &waveID)

	if string(riffID[:]) != "RIFF" || string(waveID[:]) != "WAVE" {
		return wavHeader{}, nil, fmt.Errorf("not a WAV file")
	}

	var hdr wavHeader
	var dataBytes []byte

	for {
		var chunkID [4]byte
		var chunkSize uint32
		if err := binary.Read(f, binary.LittleEndian, &chunkID); err != nil {
			break
		}
		binary.Read(f, binary.LittleEndian, &chunkSize)

		id := string(chunkID[:])
		switch id {
		case "fmt ":
			var audioFormat, blockAlign uint16
			var byteRate uint32
			binary.Read(f, binary.LittleEndian, &audioFormat)
			binary.Read(f, binary.LittleEndian, &hdr.numChannels)
			binary.Read(f, binary.LittleEndian, &hdr.sampleRate)
			binary.Read(f, binary.LittleEndian, &byteRate)
			binary.Read(f, binary.LittleEndian, &blockAlign)
			binary.Read(f, binary.LittleEndian, &hdr.bitsPerSample)
			if audioFormat != 1 {
				return wavHeader{}, nil, fmt.Errorf("not PCM (format=%d)", audioFormat)
			}
			if hdr.bitsPerSample != 16 {
				return wavHeader{}, nil, fmt.Errorf("not 16-bit (bits=%d)", hdr.bitsPerSample)
			}
			skip := int64(chunkSize) - 16
			if skip > 0 {
				f.Seek(skip, io.SeekCurrent)
			}
		case "data":
			dataBytes = make([]byte, chunkSize)
			io.ReadFull(f, dataBytes)
		default:
			f.Seek(int64(chunkSize), io.SeekCurrent)
		}
		if chunkSize%2 != 0 {
			f.Seek(1, io.SeekCurrent)
		}
	}

	if dataBytes == nil {
		return wavHeader{}, nil, fmt.Errorf("no data chunk")
	}

	numSamples := len(dataBytes) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(dataBytes[i*2:]))
	}

	return hdr, samples, nil
}

func writeWav(path string, hdr wavHeader, samples []int16) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataSize := uint32(len(samples) * 2)
	fileSize := 36 + dataSize
	blockAlign := hdr.numChannels * hdr.bitsPerSample / 8
	byteRate := hdr.sampleRate * uint32(blockAlign)

	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, fileSize)
	f.Write([]byte("WAVE"))

	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, hdr.numChannels)
	binary.Write(f, binary.LittleEndian, hdr.sampleRate)
	binary.Write(f, binary.LittleEndian, byteRate)
	binary.Write(f, binary.LittleEndian, blockAlign)
	binary.Write(f, binary.LittleEndian, hdr.bitsPerSample)

	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, dataSize)

	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	f.Write(buf)

	return nil
}
