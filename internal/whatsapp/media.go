package whatsapp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
)

var ffmpegBin = "ffmpeg"

// ConvertToOpusOgg converts an input audio file to .ogg (Opus) using ffmpeg.
// Returns the output path (temporary next to input) without removing the input.
func ConvertToOpusOgg(inputPath string) (string, error) {
	if _, err := os.Stat(inputPath); err != nil {
		return "", fmt.Errorf("input missing: %w", err)
	}
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)
	out := filepath.Join(dir, base+".converted.ogg")
	cmd := exec.Command(ffmpegBin,
		"-i", inputPath,
		"-c:a", "libopus",
		"-b:a", "32k",
		"-ar", "24000",
		"-application", "voip",
		"-vbr", "on",
		"-compression_level", "10",
		"-frame_duration", "60",
		"-y",
		out,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}
	return out, nil
}

// AnalyzeOggOpus computes duration seconds and a 64-byte waveform for WhatsApp PTT.
func AnalyzeOggOpus(data []byte) (uint32, []byte, error) {
	if len(data) < 4 || string(data[0:4]) != "OggS" {
		return 0, nil, errors.New("not an Ogg file")
	}
	var lastGranule uint64
	var sampleRate uint32 = 48000
	var preSkip uint16
	var foundHead bool

	for i := 0; i < len(data); {
		if i+27 >= len(data) {
			break
		}
		if string(data[i:i+4]) != "OggS" {
			i++
			continue
		}
		granulePos := binary.LittleEndian.Uint64(data[i+6 : i+14])
		pageSeqNum := binary.LittleEndian.Uint32(data[i+18 : i+22])
		numSegments := int(data[i+26])
		if i+27+numSegments >= len(data) {
			break
		}
		segmentTable := data[i+27 : i+27+numSegments]
		pageSize := 27 + numSegments
		for _, seg := range segmentTable {
			pageSize += int(seg)
		}
		if !foundHead && pageSeqNum <= 1 {
			pageData := data[i : i+pageSize]
			pos := bytes.Index(pageData, []byte("OpusHead"))
			if pos >= 0 && pos+16 <= len(pageData) {
				// OpusHead: Magic(8) + Version(1) + Channels(1) + PreSkip(2) + SampleRate(4)
				if pos+8+10+6 <= len(pageData) {
					preSkip = binary.LittleEndian.Uint16(pageData[pos+8+2 : pos+8+4])
					sampleRate = binary.LittleEndian.Uint32(pageData[pos+8+4 : pos+8+8])
					foundHead = true
				}
			}
		}
		if granulePos != 0 {
			lastGranule = granulePos
		}
		i += pageSize
	}

	var duration uint32
	if lastGranule > 0 {
		d := float64(lastGranule-uint64(preSkip)) / float64(sampleRate)
		duration = uint32(math.Ceil(d))
	} else {
		duration = uint32(float64(len(data)) / 2000.0)
	}
	if duration < 1 {
		duration = 1
	}
	if duration > 300 {
		duration = 300
	}
	return duration, placeholderWaveform(duration), nil
}

func placeholderWaveform(duration uint32) []byte {
	const n = 64
	wf := make([]byte, n)
	r := rand.New(rand.NewSource(int64(duration)))
	base := 35.0
	freq := float64(minInt(int(duration), 120)) / 30.0
	for i := range wf {
		pos := float64(i) / float64(n)
		val := base*math.Sin(pos*math.Pi*freq*8) + (base/2)*math.Sin(pos*math.Pi*freq*16)
		val += (r.Float64() - 0.5) * 15
		val = val*(0.7+0.3*math.Sin(pos*math.Pi)) + 50
		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}
		wf[i] = byte(val)
	}
	return wf
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
