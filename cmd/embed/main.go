// embed reads a Go source file and embeds its contents into
// src/Composition.tsx as the goCode template literal.
//
// Usage: go run ./cmd/embed path/to/file.go
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/embed path/to/file.go")
		os.Exit(1)
	}

	srcPath := os.Args[1]
	compPath := "src/Composition.tsx"

	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatalf("✗ Cannot read source file %q: %v", srcPath, err)
	}

	compBytes, err := os.ReadFile(compPath)
	if err != nil {
		log.Fatalf("✗ Cannot read %q (run from project root?): %v", compPath, err)
	}

	// Escape the Go code for embedding inside a JS template literal.
	// Order matters: backslash first, then everything else.
	goCode := string(srcBytes)
	escaped := goCode
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	escaped = strings.ReplaceAll(escaped, `${`, `\${`)

	// Replace the existing `const goCode = `...`;` block.
	// (?s) lets . match newlines; the [^`]* keeps it greedy-but-bounded.
	pattern := regexp.MustCompile("(?s)const goCode = `[^`]*`;")
	if !pattern.Match(compBytes) {
		log.Fatalf("✗ Could not find 'const goCode = `...`;' in %s", compPath)
	}

	replacement := "const goCode = `" + escaped + "`;"
	newContent := pattern.ReplaceAllLiteralString(string(compBytes), replacement)

	if err := os.WriteFile(compPath, []byte(newContent), 0644); err != nil {
		log.Fatalf("✗ Cannot write %q: %v", compPath, err)
	}

	lineCount := strings.Count(goCode, "\n") + 1
	fmt.Printf("✓ Embedded %s (%d lines) into %s\n", srcPath, lineCount, compPath)
}
