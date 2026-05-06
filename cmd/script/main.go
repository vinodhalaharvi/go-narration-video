// script reads walkthrough/script.txt and prints a clean reading copy:
// narration text only, no directives, beats separated by blank lines.
//
// Usage:
//   go run ./cmd/script              # print to stdout
//   go run ./cmd/script -o file.txt  # save to file
//
// What it strips:
//   [[title:...]], [[intro:...]], [[outro:...]]   — visual cards
//   [[speed:...]], [[pause:...]]                  — TTS-only directives
//   [[mode:...]], [[viz:...]], [[panel:...]]      — render config
//   [[file:... line:...]], [[reveal lines:...]]   — narration anchors
//   [[output text:...]]                           — visual output panel lines
//   [[line:...]]                                  — highlight markers
//
// What it keeps:
//   The narration text following each directive, grouped into beats.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func main() {
	output := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	scriptPath := "walkthrough/script.txt"
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s not found. Run 'make use-typewriter N=...' or similar first.\n", scriptPath)
		os.Exit(1)
	}

	out := io.Writer(os.Stdout)
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ create %s: %v\n", *output, err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	clean := buildReadingCopy(string(data))
	fmt.Fprint(out, clean)

	if *output != "" {
		// Print a small summary to stderr so user knows it worked
		wordCount := len(strings.Fields(clean))
		// Rough estimate: 150 words/min for typewriter-paced narration
		estSec := float64(wordCount) / 2.5 // ~150 wpm = 2.5 wps
		fmt.Fprintf(os.Stderr, "✓ saved to %s (%d words, ~%.0fs at normal pace)\n", *output, wordCount, estSec)
	}
}

// buildReadingCopy strips all directives and formats remaining narration
// as numbered beats with paragraph breaks for easy reading.
func buildReadingCopy(script string) string {
	// Pull title for the header
	title := ""
	if m := regexp.MustCompile(`\[\[title:([^\]]+)\]\]`).FindStringSubmatch(script); m != nil {
		title = strings.TrimSpace(m[1])
	}
	introText := ""
	if m := regexp.MustCompile(`\[\[intro\s+[^\]]*text:([^\]]+?)(?:\s+icon:|\]\])`).FindStringSubmatch(script); m != nil {
		introText = strings.TrimSpace(m[1])
	}

	// Strip every directive, keeping the narration text between them.
	// We do this in passes for readability.
	directives := []*regexp.Regexp{
		regexp.MustCompile(`\[\[title:[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[intro\s+[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[outro\s+[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[speed:[0-9.]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[pause:[0-9.]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[mode:[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[viz:[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[panel:[^\]]+\]\]\s*\n?`),
		regexp.MustCompile(`\[\[output\s+text:[^\]]*\]\]\s*\n?`),
	}

	// Reveal directives are special: we want to mark them as beat boundaries
	// but keep the narration that follows. Replace each with a "BEAT" sentinel.
	revealRe := regexp.MustCompile(`\[\[(?:file:[^\]]+\s+)?reveal\s+lines:[^\]]+\]\]\s*`)
	withSentinels := revealRe.ReplaceAllString(script, "‖BEAT‖")

	// Also catch plain [[file:... line:...]] markers (highlight mode)
	markerRe := regexp.MustCompile(`\[\[(?:file:[^\s\]]+\s+)?line:\d+\]\]\s*`)
	withSentinels = markerRe.ReplaceAllString(withSentinels, "‖BEAT‖")

	// Remove all the other directives
	for _, re := range directives {
		withSentinels = re.ReplaceAllString(withSentinels, "")
	}

	// Now split on the BEAT sentinel
	parts := strings.Split(withSentinels, "‖BEAT‖")

	var sb strings.Builder
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	if introText != "" {
		fmt.Fprintf(&sb, "  READING SCRIPT — %s\n", introText)
	} else if title != "" {
		fmt.Fprintf(&sb, "  READING SCRIPT — %s\n", title)
	} else {
		sb.WriteString("  READING SCRIPT\n")
	}
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	sb.WriteString("Tips: read at a natural pace, pause briefly between beats,\n")
	sb.WriteString("don't worry about exact wording — Whisper transcribes whatever\n")
	sb.WriteString("you actually say. The first 4 words of each beat should match\n")
	sb.WriteString("the script (those are the sync anchors).\n\n")

	beatNum := 0
	for i, part := range parts {
		text := normalizeWhitespace(part)
		if text == "" {
			continue
		}
		if i == 0 {
			// Pre-beat narration is the intro paragraph
			sb.WriteString("[INTRO]\n")
			sb.WriteString(text)
			sb.WriteString("\n\n")
			sb.WriteString("[Pause briefly]\n\n")
			continue
		}
		beatNum++
		fmt.Fprintf(&sb, "[BEAT %d]\n", beatNum)
		sb.WriteString(text)
		sb.WriteString("\n\n")
		// Only add pause hint if there's another beat after
		if i < len(parts)-1 {
			sb.WriteString("[Pause briefly]\n\n")
		}
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	wc := countWords(parts)
	estSec := float64(wc) / 2.5
	fmt.Fprintf(&sb, "  Total: %d words, ~%.0fs at natural pace\n", wc, estSec)
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	return sb.String()
}

func normalizeWhitespace(s string) string {
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func countWords(parts []string) int {
	n := 0
	for _, p := range parts {
		n += len(strings.Fields(normalizeWhitespace(p)))
	}
	return n
}
