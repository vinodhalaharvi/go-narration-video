// stitch.go — synthesize narration in chunks separated by silence segments.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// synthesizeWithPauses splits the cleaned narration text on the pause
// sentinels written by extractPauses, synthesizes each chunk via the TTS
// provider, and concatenates them with silence segments between, producing
// the final narration.mp3 at outputPath.
//
// Requires ffmpeg on PATH (already used by Remotion for rendering, so it's
// already a dependency of this project).
func synthesizeWithPauses(tts TTS, fullText string, pauseDurations []float64, speed float64, outputPath string) error {
	// Sentinels look like ‖PAUSE0‖, ‖PAUSE1‖ ... — split on them.
	sentinelRe := regexp.MustCompile(`‖PAUSE\d+‖`)
	chunks := sentinelRe.Split(fullText, -1)

	// chunks[0] ... chunks[N-1], with N-1 pauses between them.
	if len(chunks) != len(pauseDurations)+1 {
		return fmt.Errorf("pause sentinel count mismatch: got %d chunks, %d pauses", len(chunks), len(pauseDurations))
	}

	// Verify ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg required for pauses (already needed by Remotion)")
	}

	tempDir, err := os.MkdirTemp("", "narration-stitch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Synthesize each chunk
	chunkFiles := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk-%03d.mp3", i))
		stream, err := tts.Synthesize(context.Background(), chunk, speed)
		if err != nil {
			return fmt.Errorf("synth chunk %d: %w", i, err)
		}
		f, err := os.Create(chunkPath)
		if err != nil {
			stream.Close()
			return err
		}
		if _, err := io.Copy(f, stream); err != nil {
			stream.Close()
			f.Close()
			return err
		}
		stream.Close()
		f.Close()
		chunkFiles = append(chunkFiles, chunkPath)
		fmt.Printf("  ✓ chunk %d: %d chars\n", i+1, len(chunk))
	}

	// Generate silence files
	silenceFiles := make([]string, 0, len(pauseDurations))
	for i, dur := range pauseDurations {
		silencePath := filepath.Join(tempDir, fmt.Sprintf("silence-%03d.mp3", i))
		// ffmpeg -f lavfi -i anullsrc=r=24000:cl=mono -t DUR -q:a 9 -acodec libmp3lame OUT
		cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=24000:cl=mono",
			"-t", fmt.Sprintf("%.3f", dur),
			"-q:a", "9", "-acodec", "libmp3lame",
			"-loglevel", "error",
			silencePath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("silence gen: %w (%s)", err, string(out))
		}
		silenceFiles = append(silenceFiles, silencePath)
	}

	// Build concat list: chunk0, silence0, chunk1, silence1, ..., chunkN
	listPath := filepath.Join(tempDir, "concat.txt")
	listFile, err := os.Create(listPath)
	if err != nil {
		return err
	}
	for i, cf := range chunkFiles {
		fmt.Fprintf(listFile, "file '%s'\n", cf)
		if i < len(silenceFiles) {
			fmt.Fprintf(listFile, "file '%s'\n", silenceFiles[i])
		}
	}
	listFile.Close()

	// Concat with ffmpeg
	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-loglevel", "error",
		outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("concat: %w (%s)", err, string(out))
	}

	return nil
}
