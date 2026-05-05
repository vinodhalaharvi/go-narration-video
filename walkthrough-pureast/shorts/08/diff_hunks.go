// cmd/pureast/commands/diff_hunks.go
//
// Parse hunk headers from `git diff` output and intersect them with
// AST line ranges so `pureast diff` can emit only the symbols whose
// lines actually changed — not every symbol in any touched file.
//
// We invoke git with -U0 so each hunk header reports just the changed
// lines, no context. Format of the headers we care about:
//
//   diff --git a/path/to/file.go b/path/to/file.go
//   ...
//   @@ -OLDSTART[,OLDCOUNT] +NEWSTART[,NEWCOUNT] @@
//
// We track the "+" side because we want lines as they exist in HEAD,
// which is what we then intersect against fset positions of the AST
// declarations we parse.

package commands

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// hunkRange is a half-open [Start, End) range of 1-indexed line numbers
// in the new (HEAD) version of a file.
type hunkRange struct {
	Start, End int
}

// changedHunks runs `git diff --unified=0 ref HEAD` and parses the
// resulting hunk headers. Returns a map keyed by absolute file path,
// whose values are the line ranges that changed in HEAD.
//
// Uses --unified=0 (zero context) so hunks report only the changed
// lines, not the surrounding lines git would normally include for
// human review. This is the right choice for programmatic intersection
// — we want to know precisely which lines changed.
func changedHunks(ctx context.Context, ref, root string) (map[string][]hunkRange, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--unified=0", ref, "HEAD")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff -U0 failed: %s", strings.TrimSpace(string(out)))
	}

	hunks := map[string][]hunkRange{}

	var (
		currentFile string
		lines       = strings.Split(string(out), "\n")
	)

	for _, line := range lines {
		// Track the current file from "+++ b/path" headers. The "a/"
		// prefix belongs to the old version; we want the new path.
		if strings.HasPrefix(line, "+++ ") {
			currentFile = parseGitFilePath(line, "+++ ")
			continue
		}
		if strings.HasPrefix(line, "@@ ") && currentFile != "" {
			r, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			full := filepath.Join(root, currentFile)
			hunks[full] = append(hunks[full], r)
		}
	}

	return hunks, nil
}

// parseGitFilePath extracts the file path from a git diff header line
// like "+++ b/path/to/file.go". Returns "" for "/dev/null" (deleted
// files) which we don't want to track.
func parseGitFilePath(line, prefix string) string {
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSpace(rest)
	if rest == "/dev/null" {
		return ""
	}
	// git prefixes paths with "b/" on the new side; strip it
	rest = strings.TrimPrefix(rest, "b/")
	return rest
}

// parseHunkHeader parses "@@ -A,B +C,D @@" or "@@ -A +C @@" and returns
// the new-side range [C, C+D). The range is half-open.
//
// Special cases handled:
//   - Missing comma means count=1: "@@ -5 +7 @@" → start=7, count=1
//   - Count=0 means a deletion (no lines added on the new side); we
//     skip these because there's nothing in HEAD to intersect with.
//   - Pure-add files have "@@ -0,0 +1,N @@"; we keep these as ranges
//     starting at 1.
func parseHunkHeader(line string) (hunkRange, bool) {
	// Find the "+X[,Y]" token. The "@@" markers bracket it but we just
	// scan for the '+' that starts the new-side range.
	plus := strings.Index(line, "+")
	if plus < 0 {
		return hunkRange{}, false
	}
	// Substring from '+' up to the next space.
	rest := line[plus+1:]
	space := strings.Index(rest, " ")
	if space < 0 {
		return hunkRange{}, false
	}
	spec := rest[:space]

	startStr, countStr := spec, "1"
	if comma := strings.Index(spec, ","); comma >= 0 {
		startStr = spec[:comma]
		countStr = spec[comma+1:]
	}

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return hunkRange{}, false
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return hunkRange{}, false
	}
	if count == 0 {
		// Pure deletion on the new side — nothing to intersect.
		return hunkRange{}, false
	}
	return hunkRange{Start: start, End: start + count}, true
}

// rangeOverlaps reports whether the symbol's line range [symStart, symEnd]
// (inclusive) intersects any of the hunks. Symbol ranges come from
// fset.Position; AST gives us Pos and End, both inclusive in source.
func rangeOverlaps(symStart, symEnd int, hunks []hunkRange) bool {
	if symEnd < symStart {
		symEnd = symStart
	}
	for _, h := range hunks {
		// half-open hunks [h.Start, h.End)  vs  inclusive sym [symStart, symEnd]
		if h.Start <= symEnd && symStart < h.End {
			return true
		}
	}
	return false
}
