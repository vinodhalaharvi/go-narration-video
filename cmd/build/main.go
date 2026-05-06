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
	// Typewriter mode reveals code progressively instead of highlighting
	// pre-existing code. Empty = off (default highlight mode).
	Typewriter        string        `json:"typewriter"`        // "" | "line" | "word"
	TypewriterReveals []RevealEntry `json:"typewriterReveals"` // ordered timeline

	// Bottom-panel visualization. "" = off, "tree" = SampleTree viz.
	// VizStartSec is the time at which the viz becomes visible (typically when
	// the SampleTree definition is revealed).
	Viz         string  `json:"viz"`
	VizStartSec float64 `json:"vizStartSec"`
}

// RevealEntry describes a typewriter reveal beat.
// At time StartSec, lines [LineFrom..LineTo] of File begin animating.
// They finish by EndSec (the start of the next reveal, or end of audio).
type RevealEntry struct {
	File      string  `json:"file"`
	LineFrom  int     `json:"lineFrom"`
	LineTo    int     `json:"lineTo"`
	StartSec  float64 `json:"startSec"`
	EndSec    float64 `json:"endSec"`
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
	// Strip pause sentinels — they survive into narration text but should
	// never become part of an anchor (Whisper won't transcribe silence).
	s = regexp.MustCompile(`‖PAUSE\d+‖`).ReplaceAllString(s, "")
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

// extractMode pulls a [[mode:typewriter granularity:line|word]] directive.
// Returns ("line"|"word"|"") for granularity. Default granularity is "line".
func extractMode(script string) (mode, granularity, cleaned string) {
	modeRe := regexp.MustCompile(`\[\[mode:typewriter(?:\s+granularity:(\w+))?\]\]\s*\n?`)
	m := modeRe.FindStringSubmatch(script)
	if m == nil {
		return "", "", script
	}
	mode = "typewriter"
	granularity = m[1]
	if granularity != "word" {
		granularity = "line"
	}
	cleaned = modeRe.ReplaceAllString(script, "")
	return mode, granularity, cleaned
}

// extractSpeed pulls a [[speed:N]] directive (anywhere in script).
// Returns 0 if not present (caller treats as "use default 1.0").
// Range is clamped to OpenAI's supported 0.25–4.0.
func extractSpeed(script string) (speed float64, cleaned string) {
	speedRe := regexp.MustCompile(`\[\[speed:([0-9]*\.?[0-9]+)\]\]\s*\n?`)
	m := speedRe.FindStringSubmatch(script)
	if m == nil {
		return 0, script
	}
	fmt.Sscanf(m[1], "%f", &speed)
	if speed < 0.25 {
		speed = 0.25
	} else if speed > 4.0 {
		speed = 4.0
	}
	cleaned = speedRe.ReplaceAllString(script, "")
	return speed, cleaned
}

// extractViz pulls a [[viz:tree start-line:N]] directive.
// `start-line` tells us which reveal triggers the viz appearing — the viz
// becomes visible when the typewriter reaches the reveal that includes
// (or is just past) line N.
//
// Returns kind="" if not present.
func extractViz(script string) (kind string, startLine int, cleaned string) {
	vizRe := regexp.MustCompile(`\[\[viz:(\w+)(?:\s+start-line:(\d+))?\]\]\s*\n?`)
	m := vizRe.FindStringSubmatch(script)
	if m == nil {
		return "", 0, script
	}
	kind = m[1]
	if m[2] != "" {
		fmt.Sscanf(m[2], "%d", &startLine)
	}
	cleaned = vizRe.ReplaceAllString(script, "")
	return kind, startLine, cleaned
}

// pause marks an inserted silence between narration segments.
// Lives in the script as [[pause:N]] (seconds). The pipeline replaces the
// directive with a placeholder, generates TTS for the surrounding chunks,
// and stitches them with N seconds of silence in between.
type pauseDirective struct {
	atIndex  int     // position in the segment stream (after how many segments)
	duration float64 // seconds of silence
}

// extractPauses finds [[pause:N]] directives. They don't survive into the
// narration text — they are replaced with a marker token that we use to
// chunk the script for TTS, then we splice silence between chunks.
//
// Returns the cleaned script (with directives replaced by a sentinel) and
// the list of pause durations in script order.
//
// We use sentinel tokens like "‖PAUSE0‖", "‖PAUSE1‖" that are very unlikely
// to appear in any real narration text.
func extractPauses(script string) (cleaned string, durations []float64) {
	pauseRe := regexp.MustCompile(`\[\[pause:([0-9]*\.?[0-9]+)\]\]\s*\n?`)
	matches := pauseRe.FindAllStringSubmatchIndex(script, -1)
	if len(matches) == 0 {
		return script, nil
	}
	var b strings.Builder
	last := 0
	idx := 0
	for _, m := range matches {
		// m[0]:m[1] = full match, m[2]:m[3] = capture group
		b.WriteString(script[last:m[0]])
		var d float64
		fmt.Sscanf(script[m[2]:m[3]], "%f", &d)
		if d < 0.05 {
			d = 0.05
		} else if d > 10 {
			d = 10
		}
		durations = append(durations, d)
		// Sentinel that won't be spoken or interpreted as a marker.
		// We'll split on this later.
		fmt.Fprintf(&b, " ‖PAUSE%d‖ ", idx)
		idx++
		last = m[1]
	}
	b.WriteString(script[last:])
	return b.String(), durations
}

// reveal is a parsed [[reveal lines:N-M]] directive in typewriter mode.
type reveal struct {
	file     string
	lineFrom int
	lineTo   int
	anchor   string // first 4 words after the directive (for time matching)
}

// parseTypewriterReveals extracts [[file:X.go reveal lines:N-M]] or
// [[reveal lines:N-M]] directives. The script is the post-extractMode text.
//
// Returns the reveal list, the cleaned script (markers removed), and a flat
// list of "segments" compatible with parseMarkers' contract so audio sync
// keeps working: each reveal counts as a narration beat at the line LineFrom.
func parseTypewriterReveals(rawScript, defaultFile string) ([]reveal, []segment, string) {
	// Match either [[file:X.go reveal lines:N[-M]]] or [[reveal lines:N[-M]]]
	revealRe := regexp.MustCompile(`\[\[(?:file:(\S+)\s+)?reveal\s+lines:(\d+)(?:-(\d+))?\]\]`)
	parts := revealRe.Split(rawScript, -1)
	matches := revealRe.FindAllStringSubmatch(rawScript, -1)

	var reveals []reveal
	var segments []segment
	currentFile := defaultFile

	intro := strings.TrimSpace(parts[0])
	if intro != "" && len(matches) > 0 {
		// Intro narration before first reveal — anchor at line 1
		segments = append(segments, segment{
			file:   currentFile,
			line:   1,
			anchor: firstNWords(intro, 4),
		})
	}

	for i, m := range matches {
		fileGroup := m[1]
		fromS := m[2]
		toS := m[3]
		if fileGroup != "" {
			currentFile = fileGroup
		}
		var from, to int
		fmt.Sscanf(fromS, "%d", &from)
		if toS != "" {
			fmt.Sscanf(toS, "%d", &to)
		} else {
			to = from
		}
		text := strings.TrimSpace(parts[i+1])
		anchor := firstNWords(text, 4)
		reveals = append(reveals, reveal{
			file:     currentFile,
			lineFrom: from,
			lineTo:   to,
			anchor:   anchor,
		})
		segments = append(segments, segment{
			file:   currentFile,
			line:   from,
			anchor: anchor,
		})
	}

	cleaned := revealRe.ReplaceAllString(rawScript, "")
	return reveals, segments, cleaned
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

	// Speed: [[speed:0.85]] in script, or SPEED env override.
	speed, rawScript := extractSpeed(rawScript)
	if envSpeed := strings.TrimSpace(os.Getenv("SPEED")); envSpeed != "" {
		var s float64
		if _, err := fmt.Sscanf(envSpeed, "%f", &s); err == nil && s > 0 {
			if s < 0.25 {
				s = 0.25
			} else if s > 4.0 {
				s = 4.0
			}
			speed = s
			fmt.Printf("ℹ SPEED=%.2f env override\n", speed)
		} else {
			log.Fatalf("invalid SPEED=%q (numeric, 0.25–4.0)", envSpeed)
		}
	}
	if speed > 0 && speed != 1.0 {
		fmt.Printf("ℹ narration speed: %.2fx\n", speed)
	}
	if speed == 0 {
		speed = 1.0 // default
	}

	// Pauses: [[pause:0.5]] inserts silence between narration chunks.
	rawScript, pauseDurations := extractPauses(rawScript)
	if len(pauseDurations) > 0 {
		fmt.Printf("ℹ %d pause(s) requested\n", len(pauseDurations))
	}

	// Optional bottom-panel visualization (e.g. [[viz:tree start-line:69]])
	vizKind, vizStartLine, rawScript := extractViz(rawScript)

	// Typewriter mode: switches the rendering model entirely.
	// Reveals replace markers as the source of narration beats.
	mode, granularity, rawScript := extractMode(rawScript)

	// Env var overrides (set via Makefile variables, e.g. `make short MODE=typewriter`).
	// MODE=typewriter         — force typewriter on (script must have [[reveal ...]] directives)
	// MODE=off                — force highlight-and-scroll mode even if script has [[mode:typewriter]]
	// GRANULARITY=line|word   — override granularity (only meaningful in typewriter mode)
	if envMode := strings.TrimSpace(os.Getenv("MODE")); envMode != "" {
		switch envMode {
		case "typewriter":
			if mode != "typewriter" {
				fmt.Println("ℹ MODE=typewriter override: forcing typewriter mode")
				mode = "typewriter"
				if granularity == "" {
					granularity = "line"
				}
			}
		case "off", "highlight":
			if mode == "typewriter" {
				fmt.Println("ℹ MODE=off override: forcing highlight mode (ignoring [[mode:typewriter]])")
				mode = ""
				granularity = ""
			}
		default:
			log.Fatalf("invalid MODE=%q (use 'typewriter' or 'off')", envMode)
		}
	}
	if envGran := strings.TrimSpace(os.Getenv("GRANULARITY")); envGran != "" && mode == "typewriter" {
		switch envGran {
		case "line", "word":
			if granularity != envGran {
				fmt.Printf("ℹ GRANULARITY=%s override\n", envGran)
				granularity = envGran
			}
		default:
			log.Fatalf("invalid GRANULARITY=%q (use 'line' or 'word')", envGran)
		}
	}

	var reveals []reveal
	var segments []segment
	var cleanText string
	if mode == "typewriter" {
		reveals, segments, cleanText = parseTypewriterReveals(rawScript, defaultFile)
		// Strip whitespace and collapse the cleaned narration text
		cleanText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanText, " ")
		cleanText = strings.TrimSpace(cleanText)
		fmt.Printf("ℹ typewriter mode (granularity: %s) — %d reveal(s)\n", granularity, len(reveals))
		if len(reveals) == 0 {
			log.Fatal("typewriter mode is on but no [[reveal lines:N-M]] directives found in script")
		}
	} else {
		segments, cleanText = parseMarkers(rawScript, defaultFile)
	}
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

	os.MkdirAll("public", 0755)

	// If pauses are present, split cleanText on the sentinels, synthesize each
	// chunk, and stitch with silence in between.
	if len(pauseDurations) > 0 {
		if err := synthesizeWithPauses(tts, cleanText, pauseDurations, speed, "public/narration.mp3"); err != nil {
			log.Fatalf("TTS with pauses failed: %v", err)
		}
	} else {
		audioStream, err := tts.Synthesize(context.Background(), cleanText, speed)
		if err != nil {
			log.Fatalf("TTS request failed: %v", err)
		}
		defer audioStream.Close()
		out, err := os.Create("public/narration.mp3")
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(out, audioStream)
		out.Close()
	}
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

	// Build typewriter reveal timeline.
	// Each reveal's StartSec = anchor time of that beat in the schedule.
	// EndSec = StartSec of next beat (or total duration if last).
	var typewriterReveals []RevealEntry
	if mode == "typewriter" && len(reveals) > 0 {
		// Walk schedule entries that correspond to reveal segments.
		// We rely on the reveal/segment 1:1 ordering: reveals[i] is at the same
		// place in segments as schedule[j] where j tracks matched anchors.
		// Find timing for each reveal by re-matching its anchor.
		searchFrom := 0
		revealStarts := make([]float64, len(reveals))
		for i, r := range reveals {
			startSec, nextIdx := findPhraseStart(words, r.anchor, searchFrom)
			if startSec < 0 {
				fmt.Printf("⚠ couldn't locate reveal anchor %q for %s:%d-%d\n", r.anchor, r.file, r.lineFrom, r.lineTo)
				revealStarts[i] = -1
				continue
			}
			revealStarts[i] = startSec
			searchFrom = nextIdx
		}
		totalDur := transcriptResp.Duration + 0.5
		for i, r := range reveals {
			if revealStarts[i] < 0 {
				continue
			}
			end := totalDur
			if i+1 < len(reveals) && revealStarts[i+1] >= 0 {
				end = revealStarts[i+1]
			}
			typewriterReveals = append(typewriterReveals, RevealEntry{
				File:     r.file,
				LineFrom: r.lineFrom,
				LineTo:   r.lineTo,
				StartSec: revealStarts[i],
				EndSec:   end,
			})
		}
	}

	// Compute viz start time: find the reveal that contains or starts at vizStartLine.
	var vizStartSec float64
	if vizKind != "" && vizStartLine > 0 {
		for _, r := range typewriterReveals {
			if r.LineFrom <= vizStartLine && vizStartLine <= r.LineTo {
				vizStartSec = r.StartSec
				break
			}
			if r.LineFrom >= vizStartLine {
				vizStartSec = r.StartSec
				break
			}
		}
	}

	meta := Meta{
		DurationSec:       transcriptResp.Duration + 0.5,
		Format:            format,
		Title:             title,
		IntroIcon:         introIcon,
		IntroText:         introText,
		Typewriter:        granularity, // "" if not typewriter mode
		TypewriterReveals: typewriterReveals,
		Viz:               vizKind,
		VizStartSec:       vizStartSec,
	}
	if mode != "typewriter" {
		meta.Typewriter = ""
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile("src/meta.json", metaBytes, 0644)

	// --- Console summary ---
	fmt.Printf("✓ schedule:\n")
	for _, s := range schedule {
		fmt.Printf("   %s:%d at %.2fs\n", s.File, s.Line, s.StartSec)
	}
	if len(typewriterReveals) > 0 {
		fmt.Printf("✓ typewriter reveals:\n")
		for _, r := range typewriterReveals {
			fmt.Printf("   %s:%d-%d at %.2f-%.2fs\n", r.File, r.LineFrom, r.LineTo, r.StartSec, r.EndSec)
		}
	}
	fmt.Printf("✓ duration: %.2fs (video will end here)\n", meta.DurationSec)
	fmt.Printf("✓ format: %s", meta.Format)
	if meta.Title != "" {
		fmt.Printf(" (title: %q)", meta.Title)
	}
	if meta.IntroText != "" {
		fmt.Printf(" (intro: %s %q)", meta.IntroIcon, meta.IntroText)
	}
	if meta.Typewriter != "" {
		fmt.Printf(" (typewriter: %s)", meta.Typewriter)
	}
	fmt.Println()

	// YouTube Shorts limit was bumped to 3 minutes (180s) on Oct 15, 2024.
	if format == "short" {
		switch {
		case meta.DurationSec > 180:
			fmt.Printf("⚠ WARNING: %.1fs exceeds YouTube Shorts 180s (3 min) limit. YouTube will treat as regular video.\n", meta.DurationSec)
		case meta.DurationSec > 60:
			fmt.Printf("ℹ %.1fs is over 60s — still a Short (max is 180s), but retention drops sharply past 60s. Sweet spot is 20-45s.\n", meta.DurationSec)
		}
	}
}
