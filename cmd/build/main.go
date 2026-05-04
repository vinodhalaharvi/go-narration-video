package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type ScheduleEntry struct {
	Line     int     `json:"line"`
	StartSec float64 `json:"startSec"`
}

type Meta struct {
	DurationSec float64 `json:"durationSec"`
}

type Word struct {
	Word  string
	Start float64
	End   float64
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9 ]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func findPhraseStart(words []Word, phrase string, fromIdx int) (float64, int) {
	target := strings.Fields(normalize(phrase))
	if len(target) == 0 {
		return -1, fromIdx
	}

	for i := fromIdx; i <= len(words)-len(target); i++ {
		match := true
		for j := 0; j < len(target); j++ {
			w := normalize(words[i+j].Word)
			if w != target[j] {
				match = false
				break
			}
		}
		if match {
			return words[i].Start, i + len(target)
		}
	}
	return -1, fromIdx
}

func firstNWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
}

func main() {
	raw, err := os.ReadFile("script.txt")
	if err != nil {
		log.Fatal(err)
	}
	rawStr := string(raw)

	markerRe := regexp.MustCompile(`\[\[line:(\d+)\]\]`)

	type segment struct {
		line   int
		anchor string
	}
	var segments []segment

	parts := markerRe.Split(rawStr, -1)
	matches := markerRe.FindAllStringSubmatch(rawStr, -1)

	intro := strings.TrimSpace(parts[0])
	if intro != "" {
		segments = append(segments, segment{line: 1, anchor: firstNWords(intro, 4)})
	}

	for i, m := range matches {
		lineNum := 0
		fmt.Sscanf(m[1], "%d", &lineNum)
		text := strings.TrimSpace(parts[i+1])
		segments = append(segments, segment{line: lineNum, anchor: firstNWords(text, 4)})
	}

	cleanText := markerRe.ReplaceAllString(rawStr, "")
	cleanText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanText, " ")

	// --- Generate audio ---
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	resp, err := client.CreateSpeech(context.Background(), openai.CreateSpeechRequest{
		Model: "gpt-4o-mini-tts",
		Input: cleanText,
		Voice: openai.VoiceNova,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Close()

	os.MkdirAll("public", 0755)
	out, err := os.Create("public/narration.mp3")
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(out, resp)
	out.Close()
	fmt.Println("✓ audio generated")

	// --- Transcribe ---
	transcriptResp, err := client.CreateTranscription(context.Background(), openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: "public/narration.mp3",
		Format:   openai.AudioResponseFormatVerboseJSON,
		TimestampGranularities: []openai.TranscriptionTimestampGranularity{
			openai.TranscriptionTimestampGranularityWord,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	var words []Word
	for _, w := range transcriptResp.Words {
		words = append(words, Word{
			Word:  w.Word,
			Start: w.Start,
			End:   w.End,
		})
	}
	fmt.Printf("✓ %d words timestamped\n", len(words))

	// --- Match anchors against word stream ---
	schedule := []ScheduleEntry{}
	searchFrom := 0
	for _, seg := range segments {
		startSec, nextIdx := findPhraseStart(words, seg.anchor, searchFrom)
		if startSec < 0 {
			fmt.Printf("⚠ couldn't locate anchor %q for line %d\n", seg.anchor, seg.line)
			continue
		}
		schedule = append(schedule, ScheduleEntry{
			Line:     seg.line,
			StartSec: startSec,
		})
		searchFrom = nextIdx
	}

	if len(schedule) == 0 || schedule[0].StartSec > 0.1 {
		schedule = append([]ScheduleEntry{{Line: 1, StartSec: 0}}, schedule...)
	}

	os.MkdirAll("src", 0755)
	scheduleBytes, _ := json.MarshalIndent(schedule, "", "  ")
	os.WriteFile("src/schedule.json", scheduleBytes, 0644)

	// Write meta.json with audio duration so Root.tsx sizes the video to fit narration.
	// Add 0.5s tail buffer so the last word isn't cut off and audio fully decays.
	meta := Meta{DurationSec: transcriptResp.Duration + 0.5}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile("src/meta.json", metaBytes, 0644)

	fmt.Printf("✓ schedule:\n")
	for _, s := range schedule {
		fmt.Printf("   line %d at %.2fs\n", s.Line, s.StartSec)
	}
	fmt.Printf("✓ duration: %.2fs (video will end here)\n", meta.DurationSec)
}

