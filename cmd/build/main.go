// build reads walkthrough/*.go and walkthrough/script.txt,
// generates narration audio, transcribes for word timestamps,
// and writes schedule.json + meta.json + codeFiles.json + captions.json.
//
// Marker syntax in script.txt:
//   [[title:Hook text]]              — overlay shown for shorts (not spoken)
//   [[file:functor.go line:6]]       — switch to file + highlight line
//   [[line:14]]                      — highlight line in current file
//
// Env vars:
//   SHORT=1   — produce a vertical short (9:16) with baked captions
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type ScheduleEntry struct {
	File     string  `json:"file"`
	Line     int     `json:"line"`
	StartSec float64 `json:"startSec"`
}

type Meta struct {
	DurationSec float64 `json:"durationSec"`
	Format      string  `json:"format"` // "long" or "short"
	Title       string  `json:"title"`
	IntroIcon   string  `json:"introIcon"`
	IntroText   string  `json:"introText"`
}

type CaptionWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Word struct {
	Word  string
	Start float64
	End   float64
}

type segment struct {
	file   string
	line   int
	anchor string
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
			if normalize(words[i+j].Word) != target[j] {
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

// loadCodeFiles walks walkthrough/ for .go files. Returns empty if dir missing.
func loadCodeFiles() (map[string]string, []string, error) {
	files := make(map[string]string)
	var names []string

	dir := "walkthrough"
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return files, names, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
		files[e.Name()] = string(data)
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return files, names, nil
}

// loadScript prefers walkthrough/script.txt, falls back to ./script.txt.
func loadScript() (string, error) {
	for _, p := range []string{"walkthrough/script.txt", "script.txt"} {
		if data, err := os.ReadFile(p); err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("no script.txt found in walkthrough/ or project root")
}

// extractTitle pulls a [[title:...]] directive (not spoken — only shown as overlay).
func extractTitle(script string) (title, cleaned string) {
	titleRe := regexp.MustCompile(`\[\[title:([^\]]+)\]\]\s*\n?`)
	if m := titleRe.FindStringSubmatch(script); m != nil {
		title = strings.TrimSpace(m[1])
	}
	cleaned = titleRe.ReplaceAllString(script, "")
	return title, cleaned
}

// extractIntro pulls an [[intro icon:X text:Y]] directive (shorts only).
func extractIntro(script string) (icon, text, cleaned string) {
	introRe := regexp.MustCompile(`\[\[intro\s+([^\]]+)\]\]\s*\n?`)
	m := introRe.FindStringSubmatch(script)
	if m == nil {
		return "", "", script
	}
	body := m[1]

	iconRe := regexp.MustCompile(`icon:(\S+)`)
	if im := iconRe.FindStringSubmatch(body); im != nil {
		icon = strings.TrimSpace(im[1])
	}

	textRe := regexp.MustCompile(`text:(.+?)(?:\s+icon:|$)`)
	if tm := textRe.FindStringSubmatch(body); tm != nil {
		text = strings.TrimSpace(tm[1])
	}

	cleaned = introRe.ReplaceAllString(script, "")
	return icon, text, cleaned
}

// parseMarkers extracts segments. Supports:
//   [[file:foo.go line:6]] — explicit
//   [[line:14]]            — inherits last file
func parseMarkers(rawScript, defaultFile string) ([]segment, string) {
	markerRe := regexp.MustCompile(`\[\[(?:file:([^\s\]]+)\s+)?line:(\d+)\]\]`)
	parts := markerRe.Split(rawScript, -1)
	matches := markerRe.FindAllStringSubmatch(rawScript, -1)

	var segments []segment
	currentFile := defaultFile

	intro := strings.TrimSpace(parts[0])
	if intro != "" {
		segments = append(segments, segment{
			file:   currentFile,
			line:   1,
			anchor: firstNWords(intro, 4),
		})
	}

	for i, m := range matches {
		fileGroup := m[1]
		lineNum := 0
		fmt.Sscanf(m[2], "%d", &lineNum)

		if fileGroup != "" {
			currentFile = fileGroup
		}
		text := strings.TrimSpace(parts[i+1])
		segments = append(segments, segment{
			file:   currentFile,
			line:   lineNum,
			anchor: firstNWords(text, 4),
		})
	}

	cleanText := markerRe.ReplaceAllString(rawScript, "")
	cleanText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanText, " ")
	cleanText = strings.TrimSpace(cleanText)

	return segments, cleanText
}

func main() {
	codeFiles, fileNames, err := loadCodeFiles()
	if err != nil {
		log.Fatalf("loading walkthrough: %v", err)
	}

	defaultFile := ""
	if len(fileNames) > 0 {
		defaultFile = fileNames[0]
		fmt.Printf("✓ loaded %d code file(s): %s\n", len(fileNames), strings.Join(fileNames, ", "))
	} else {
		defaultFile = "main.go"
		fmt.Println("ℹ no walkthrough/ dir — using legacy single-file mode")
	}

	rawScript, err := loadScript()
	if err != nil {
		log.Fatal(err)
	}

	title, rawScript := extractTitle(rawScript)
	introIcon, introText, rawScript := extractIntro(rawScript)
	segments, cleanText := parseMarkers(rawScript, defaultFile)
	if len(segments) == 0 {
		log.Fatal("no narration segments found in script.txt")
	}

	format := "long"
	if os.Getenv("SHORT") == "1" {
		format = "short"
		fmt.Println("ℹ shorts mode: 9:16 vertical with baked captions")

		if introText == "" && title != "" {
			introText = title
			fmt.Printf("ℹ auto-generated intro from title: %q\n", introText)
		}
		if introText != "" && introIcon == "" {
			introIcon = "💡"
			fmt.Printf("ℹ default intro icon: %s\n", introIcon)
		}
	}

	// --- Generate audio ---
	tts, err := NewTTS()
	if err != nil {
		log.Fatalf("TTS setup failed: %v", err)
	}
	fmt.Printf("✓ using TTS: %s\n", tts.Name())

	audioStream, err := tts.Synthesize(context.Background(), cleanText)
	if err != nil {
		log.Fatalf("TTS request failed: %v", err)
	}
	defer audioStream.Close()

	os.MkdirAll("public", 0755)
	out, err := os.Create("public/narration.mp3")
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(out, audioStream)
	out.Close()
	fmt.Println("✓ audio generated")

	// --- Transcribe (always uses OpenAI Whisper for word timestamps) ---
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	transcriptResp, err := client.CreateTranscription(context.Background(), openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: "public/narration.mp3",
		Format:   openai.AudioResponseFormatVerboseJSON,
		TimestampGranularities: []openai.TranscriptionTimestampGranularity{
			openai.TranscriptionTimestampGranularityWord,
		},
	})
	if err != nil {
		log.Fatalf("transcription failed: %v", err)
	}

	var words []Word
	for _, w := range transcriptResp.Words {
		words = append(words, Word{Word: w.Word, Start: w.Start, End: w.End})
	}
	fmt.Printf("✓ %d words timestamped\n", len(words))

	// --- Match anchors against word stream ---
	schedule := []ScheduleEntry{}
	searchFrom := 0
	for _, seg := range segments {
		startSec, nextIdx := findPhraseStart(words, seg.anchor, searchFrom)
		if startSec < 0 {
			fmt.Printf("⚠ couldn't locate anchor %q for %s:%d\n", seg.anchor, seg.file, seg.line)
			continue
		}
		schedule = append(schedule, ScheduleEntry{
			File:     seg.file,
			Line:     seg.line,
			StartSec: startSec,
		})
		searchFrom = nextIdx
	}

	if len(schedule) == 0 || schedule[0].StartSec > 0.1 {
		schedule = append([]ScheduleEntry{{File: defaultFile, Line: 1, StartSec: 0}}, schedule...)
	}

	// --- Write outputs ---
	os.MkdirAll("src", 0755)

	scheduleBytes, _ := json.MarshalIndent(schedule, "", "  ")
	os.WriteFile("src/schedule.json", scheduleBytes, 0644)

	codeFilesBytes, _ := json.MarshalIndent(codeFiles, "", "  ")
	os.WriteFile("src/codeFiles.json", codeFilesBytes, 0644)

	captions := make([]CaptionWord, 0, len(words))
	for _, w := range words {
		captions = append(captions, CaptionWord{Word: w.Word, Start: w.Start, End: w.End})
	}
	captionsBytes, _ := json.MarshalIndent(captions, "", "  ")
	os.WriteFile("src/captions.json", captionsBytes, 0644)

	meta := Meta{
		DurationSec: transcriptResp.Duration + 0.5,
		Format:      format,
		Title:       title,
		IntroIcon:   introIcon,
		IntroText:   introText,
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile("src/meta.json", metaBytes, 0644)

	// --- Console summary ---
	fmt.Printf("✓ schedule:\n")
	for _, s := range schedule {
		fmt.Printf("   %s:%d at %.2fs\n", s.File, s.Line, s.StartSec)
	}
	fmt.Printf("✓ duration: %.2fs (video will end here)\n", meta.DurationSec)
	fmt.Printf("✓ format: %s", meta.Format)
	if meta.Title != "" {
		fmt.Printf(" (title: %q)", meta.Title)
	}
	if meta.IntroText != "" {
		fmt.Printf(" (intro: %s %q)", meta.IntroIcon, meta.IntroText)
	}
	fmt.Println()

	if format == "short" && meta.DurationSec > 60 {
		fmt.Printf("⚠ WARNING: %.1fs exceeds YouTube Shorts 60s limit. YouTube will treat as regular video.\n", meta.DurationSec)
	}
}
