package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

func rmsDbfs(path string) (float64, float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	f.Seek(44, io.SeekStart)
	var sum float64
	var peak float64
	var count int
	buf := make([]byte, 8192)
	for {
		n, err := f.Read(buf)
		for i := 0; i+1 < n; i += 2 {
			s := float64(int16(binary.LittleEndian.Uint16(buf[i:])))
			sum += s * s
			a := math.Abs(s)
			if a > peak {
				peak = a
			}
			count++
		}
		if err != nil {
			break
		}
	}
	if count == 0 {
		return -999, -999, nil
	}
	rms := math.Sqrt(sum / float64(count))
	rmsDb := 20 * math.Log10(rms / 32768.0)
	peakDb := 20 * math.Log10(peak / 32768.0)
	return rmsDb, peakDb, nil
}

func main() {
	files := []struct{ label, path string }{
		{"input", "test_input/room_recording.wav"},
		{"passthrough", "test_output_passthrough/room_recording.wav"},
		{"NS-only (lv2)", "test_output_ns_only/room_recording.wav"},
		{"AGC2-only", "test_output_agc_only/room_recording.wav"},
		{"NS+AGC2 (lv2)", "test_output_room/room_recording.wav"},
	}
	fmt.Printf("%-20s %10s %10s\n", "variant", "RMS dBFS", "Peak dBFS")
	fmt.Println("---------------------------------------------")
	for _, f := range files {
		rms, peak, err := rmsDbfs(f.path)
		if err != nil {
			fmt.Printf("%-20s %v\n", f.label, err)
			continue
		}
		fmt.Printf("%-20s %10.1f %10.1f\n", f.label, rms, peak)
	}
}
