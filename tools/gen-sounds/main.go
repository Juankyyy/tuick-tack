// Command gen-sounds synthesizes the completion sound pack into assets/sounds.
//
// Every sound is original: bell, marimba, synth beeps and a low gong built from
// sine partials with exponential decay. No copyrighted samples are used. The
// generator is deterministic, so `go generate ./...` must not change committed
// files unless a recipe changes.
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
	bitDepth   = 16
)

type note struct {
	start  float64
	freq   float64
	gain   float64
	decay  float64
	length float64
}

type partial struct {
	mult float64
	amp  float64
}

func main() {
	root := repoRoot()
	sounds := []struct {
		name    string
		samples []float64
	}{
		{name: "chime", samples: synthChime()},
		{name: "marimba", samples: synthMarimba()},
		{name: "digital", samples: synthDigital()},
		{name: "deep", samples: synthDeep()},
	}

	outDir := filepath.Join(root, "assets", "sounds")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}
	for _, s := range sounds {
		path := filepath.Join(outDir, s.name+".wav")
		if err := writeWAV(path, s.samples); err != nil {
			fail(err)
		}
		fmt.Printf("gen-sounds: wrote %s (%.1fs, %d KB)\n",
			path, float64(len(s.samples))/sampleRate, len(s.samples)*bitDepth/8/1024)
	}
}

// synthChime is the original four-second bell arpeggio. The arithmetic matches
// the previous generator exactly so the shipped chime does not change.
func synthChime() []float64 {
	const duration = 4.0
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
			partials := []partial{
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
	return samples
}

// synthMarimba is a warm wooden mallet: a rising C-major figure with a short
// attack and the characteristic fourth partial of a marimba bar.
func synthMarimba() []float64 {
	const duration = 3.0
	notes := []note{
		{start: 0.00, freq: 523.25, gain: 0.55, decay: 0.52, length: 2.0}, // C5
		{start: 0.24, freq: 659.25, gain: 0.46, decay: 0.46, length: 1.8}, // E5
		{start: 0.48, freq: 783.99, gain: 0.44, decay: 0.42, length: 1.8}, // G5
		{start: 0.78, freq: 1046.5, gain: 0.32, decay: 0.38, length: 2.0}, // C6
	}
	partials := []partial{
		{1.000, 1.00},
		{3.930, 0.26},
		{9.200, 0.07},
	}
	samples := renderNotes(notes, partials, 0.004, 0.035, 1.33)
	finalize(samples, 0.65, 0.55)
	return samples
}

// synthDigital is a clean four-note synth motif: short odd-harmonic beeps
// with a quiet sub-octave for body.
func synthDigital() []float64 {
	base := []note{
		{start: 0.00, freq: 880.00, gain: 0.30, decay: 0.18, length: 0.32},  // A5
		{start: 0.28, freq: 1108.73, gain: 0.29, decay: 0.18, length: 0.32}, // C#6
		{start: 0.56, freq: 1318.51, gain: 0.28, decay: 0.24, length: 0.40}, // E6
		{start: 0.86, freq: 1760.00, gain: 0.22, decay: 0.30, length: 0.80}, // A6
	}
	notes := make([]note, 0, len(base)*2)
	for _, n := range base {
		notes = append(notes, n)
		sub := n
		sub.freq /= 2
		sub.gain *= 0.28
		sub.decay *= 0.8
		sub.length *= 0.8
		notes = append(notes, sub)
	}
	partials := []partial{
		{1.000, 1.00},
		{3.000, 0.22},
		{5.000, 0.08},
		{7.000, 0.03},
	}
	samples := renderNotes(notes, partials, 0.006, 0.025, 1.33)
	finalize(samples, 0.65, 0.45)
	return samples
}

// synthDeep is a low gong: a long C3 fundamental with inharmonic partials and
// a quiet high strike that fades almost immediately.
func synthDeep() []float64 {
	notes := []note{
		{start: 0.00, freq: 130.81, gain: 0.55, decay: 1.75, length: 4.0},  // C3
		{start: 0.14, freq: 196.00, gain: 0.32, decay: 1.45, length: 3.6},  // G3
		{start: 0.28, freq: 261.63, gain: 0.20, decay: 1.20, length: 3.2},  // C4
		{start: 0.00, freq: 523.25, gain: 0.06, decay: 0.12, length: 0.55}, // Strike
	}
	partials := []partial{
		{1.000, 1.00},
		{2.010, 0.42},
		{3.030, 0.22},
		{4.120, 0.13},
		{5.430, 0.07},
	}
	samples := renderNotes(notes, partials, 0.012, 0.05, 1.84)
	finalize(samples, 0.65, 0.90)
	return samples
}

// renderNotes mixes the notes into a new buffer. norm compensates for the sum
// of the partial amplitudes so a recipe's gain is roughly its peak level.
func renderNotes(notes []note, partials []partial, attack, tail, norm float64) []float64 {
	duration := 0.0
	for _, n := range notes {
		if end := n.start + n.length; end > duration {
			duration = end
		}
	}
	total := int(sampleRate * duration)
	samples := make([]float64, total)

	for _, n := range notes {
		start := int(n.start * sampleRate)
		length := int(n.length * sampleRate)
		for i := 0; i < length && start+i < total; i++ {
			t := float64(i) / sampleRate
			env := math.Exp(-t / n.decay)
			env *= math.Min(1, t/attack)
			if tail > 0 && t > n.length-tail {
				env *= (n.length - t) / tail
			}
			var v float64
			for _, p := range partials {
				v += p.amp * math.Sin(2*math.Pi*n.freq*p.mult*t)
			}
			samples[start+i] += v * env * n.gain / norm
		}
	}
	return samples
}

// finalize fades the tail, normalizes the peak and soft-clips the result.
func finalize(samples []float64, target, fadeSeconds float64) {
	fadeSamples := int(fadeSeconds * sampleRate)
	total := len(samples)
	for i := 0; i < total; i++ {
		if i > total-fadeSamples && fadeSamples > 0 {
			f := float64(total-i) / float64(fadeSamples)
			samples[i] *= f * f
		}
	}
	peak := 0.0
	for _, s := range samples {
		if a := math.Abs(s); a > peak {
			peak = a
		}
	}
	if peak > 0 {
		scale := target / peak
		for i := range samples {
			samples[i] *= scale
		}
	}
	for i, s := range samples {
		if s > 0.95 {
			samples[i] = 0.95
		}
		if s < -0.95 {
			samples[i] = -0.95
		}
	}
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
	binary.LittleEndian.PutUint16(buf[22:], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:], sampleRate*bitDepth/8)
	binary.LittleEndian.PutUint16(buf[32:], bitDepth/8)
	binary.LittleEndian.PutUint16(buf[34:], bitDepth)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataSize))

	for i, s := range samples {
		v := int16(math.Round(s * 32767))
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(v))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-sounds:", err)
	os.Exit(1)
}
