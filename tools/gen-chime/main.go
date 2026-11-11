// Command gen-chime synthesizes the completion chime into assets/chime.wav.
//
// The sound is an original bell arpeggio: four notes, inharmonic partials,
// exponential decay and a short fade-out. No copyrighted samples are used.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	sampleRate = 44100
	duration   = 4.0
	channels   = 1
	bitDepth   = 16
)

type note struct {
	start  float64
	freq   float64
	gain   float64
	decay  float64
	length float64
}

func main() {
	out := filepath.Join(repoRoot(), "assets", "chime.wav")
	notes := []note{
		{start: 0.00, freq: 523.25, gain: 0.50, decay: 0.55, length: 2.6}, // C5
		{start: 0.32, freq: 659.25, gain: 0.46, decay: 0.50, length: 2.5}, // E5
		{start: 0.64, freq: 783.99, gain: 0.44, decay: 0.48, length: 2.4}, // G5
		{start: 1.05, freq: 1046.5, gain: 0.34, decay: 0.42, length: 2.7}, // C6
		{start: 1.50, freq: 1318.5, gain: 0.16, decay: 0.35, length: 2.2}, // E6 glow
	}

	total := int(sampleRate * duration)
	samples := make([]float64, total)

	for _, n := range notes {
		start := int(n.start * sampleRate)
		length := int(n.length * sampleRate)
		for i := 0; i < length && start+i < total; i++ {
			t := float64(i) / sampleRate
			env := math.Exp(-t / n.decay)
			attack := math.Min(1, t/0.008)
			// Slight high partial drift keeps the bell alive without beating.
			partials := []struct {
				mult float64
				amp  float64
			}{
				{1.000, 1.00},
				{2.003, 0.38},
				{3.008, 0.19},
				{4.012, 0.09},
				{5.020, 0.04},
			}
			var v float64
			for _, p := range partials {
				v += p.amp * math.Sin(2*math.Pi*n.freq*p.mult*t)
			}
			samples[start+i] += v * env * attack * n.gain / 1.7
		}
	}

	// Gentle fade-out and soft clip.
	fadeSamples := int(0.6 * sampleRate)
	for i := 0; i < total; i++ {
		if i > total-fadeSamples {
			f := float64(total-i) / float64(fadeSamples)
			samples[i] *= f * f
		}
		if samples[i] > 0.95 {
			samples[i] = 0.95
		}
		if samples[i] < -0.95 {
			samples[i] = -0.95
		}
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gen-chime:", err)
		os.Exit(1)
	}
	if err := writeWAV(out, samples); err != nil {
		fmt.Fprintln(os.Stderr, "gen-chime:", err)
		os.Exit(1)
	}
	fmt.Printf("gen-chime: wrote %s (%d samples, %.1fs)\n", out, total, duration)
}

func writeWAV(path string, samples []float64) error {
	dataSize := len(samples) * bitDepth / 8
	buf := make([]byte, 44+dataSize)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataSize))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:], channels)
	binary.LittleEndian.PutUint32(buf[24:], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:], sampleRate*channels*bitDepth/8)
	binary.LittleEndian.PutUint16(buf[32:], channels*bitDepth/8)
	binary.LittleEndian.PutUint16(buf[34:], bitDepth)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))

	for i, s := range samples {
		v := int16(math.Round(s * 32767))
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(v))
	}
	return os.WriteFile(path, buf, 0o644)
}

// repoRoot walks up from the working directory until it finds go.mod, so the
// generator writes to the repository assets/ directory regardless of where it
// is invoked from.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
