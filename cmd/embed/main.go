// embed copies one or more Go source files into walkthrough/ for the next build.
//
// Usage:
//   go run ./cmd/embed file.go [more.go ...]
//   go run ./cmd/embed --dir path/to/dir
//   go run ./cmd/embed --clear
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const walkthroughDir = "walkthrough"

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/embed file.go [more.go ...]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/embed --dir path/to/dir")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/embed --clear")
	os.Exit(1)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func clearWalkthrough() {
	entries, err := os.ReadDir(walkthroughDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("ℹ walkthrough/ doesn't exist — nothing to clear")
			return
		}
		log.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if err := os.Remove(filepath.Join(walkthroughDir, e.Name())); err != nil {
			log.Fatal(err)
		}
		count++
	}
	fmt.Printf("✓ Removed %d .go file(s) from %s/\n", count, walkthroughDir)
}

func embedDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("✗ Cannot read directory %q: %v", dir, err)
	}
	var goFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".go") {
			goFiles = append(goFiles, filepath.Join(dir, e.Name()))
		}
	}
	if len(goFiles) == 0 {
		log.Fatalf("✗ No .go files in %q", dir)
	}
	return goFiles
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
	}

	if args[0] == "--clear" {
		clearWalkthrough()
		return
	}

	var srcFiles []string
	if args[0] == "--dir" {
		if len(args) != 2 {
			usage()
		}
		srcFiles = embedDir(args[1])
	} else {
		srcFiles = args
	}

	for _, f := range srcFiles {
		info, err := os.Stat(f)
		if err != nil {
			log.Fatalf("✗ Cannot read %q: %v", f, err)
		}
		if info.IsDir() {
			log.Fatalf("✗ %q is a directory — use --dir", f)
		}
		if !strings.HasSuffix(f, ".go") {
			log.Fatalf("✗ %q is not a .go file", f)
		}
	}

	if err := os.MkdirAll(walkthroughDir, 0755); err != nil {
		log.Fatal(err)
	}

	for _, src := range srcFiles {
		dst := filepath.Join(walkthroughDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			log.Fatalf("✗ Cannot copy %s → %s: %v", src, dst, err)
		}
		data, _ := os.ReadFile(dst)
		lineCount := strings.Count(string(data), "\n") + 1
		fmt.Printf("✓ Copied %s (%d lines) → %s\n", src, lineCount, dst)
	}

	fmt.Println()
	fmt.Println("Next:")
	fmt.Println("  1. Edit walkthrough/script.txt with your narration")
	fmt.Println("  2. Use [[file:NAME line:N]] markers to reference specific files")
	fmt.Println("     Example: [[file:functor.go line:6]] We define the Map function")
	fmt.Println("  3. Run: make build  (or 'make short' for vertical 9:16)")
}
