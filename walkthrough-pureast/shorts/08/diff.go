// cmd/pureast/commands/diff.go
//
// Extract symbols in files that have changed since a given git ref.
// Intended use: PR-review and "what's new" LLM context. Instead of
// dumping the whole package, dump only what touched code in this
// branch.
//
// Strategy: shell out to `git diff --name-only <ref> HEAD` to find
// changed Go files, then run the same symbol collection as `dump`
// against just those files (or restricted to their content).
// Symbols outside changed files are excluded entirely.
//
// Limitations to call out:
//   - We do file-level granularity, not line-level. A symbol in a
//     changed file is included even if it wasn't itself modified.
//     Line-level filtering would need diff parsing and AST line-range
//     intersection — useful follow-up, not in this first cut.
//   - We don't handle deleted files: a deleted symbol can't appear
//     in a dump because the AST no longer contains it. That's
//     probably the right behavior for context generation, but would
//     matter for review summaries.

package commands

import (
	"context"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pure-Company/pureast/pkg/cli"
	"github.com/Pure-Company/pureast/pkg/extract"
	"github.com/spf13/cobra"
)

type DiffArgs struct {
	FilePath   string
	Ref        string
	OutputFile string
	Format     string // go|md
	Bodies     bool
	MaxTokens  int
	WholeFile  bool // if true, include all symbols from changed files; default = only changed-line symbols
}

func NewDiffCommand() *cobra.Command {
	cmd := cli.NewCommand[DiffArgs]("diff").
		Short("Dump symbols from files changed since a git ref").
		Long(`Extract every symbol in Go files that have changed since the given git ref.
The intended workflow is PR review: feed an LLM only the code that's new in
this branch, not the entire repo.

The ref can be any git revision (branch, tag, commit, HEAD~N).

Examples:
  pureast diff main
  pureast diff main ./pkg
  pureast diff HEAD~5 --bodies
  pureast diff origin/main --format md -o pr-context.md`).
		ParseArgs(parseDiffArgs).
		Action(diffAction).
		Build()

	cmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	cmd.Flags().String("format", "go", "Output format: go|md")
	cmd.Flags().Bool("bodies", false, "Include function bodies")
	cmd.Flags().Int("max-tokens", 0, "Truncate output to fit token budget (0 = unbounded)")
	cmd.Flags().Bool("whole-file", false,
		"Include every symbol from changed files (legacy behavior). "+
			"Default: only symbols whose lines actually changed.")

	return cmd
}

func parseDiffArgs(cmd *cobra.Command, args []string) (DiffArgs, error) {
	if len(args) < 1 {
		return DiffArgs{}, fmt.Errorf("requires REF [PATH]")
	}
	if len(args) > 2 {
		return DiffArgs{}, fmt.Errorf("expected REF [PATH], got %d args", len(args))
	}

	ref := args[0]
	path := "."
	if len(args) == 2 {
		path = args[1]
	}

	output, _ := cmd.Flags().GetString("output")
	format, _ := cmd.Flags().GetString("format")
	bodies, _ := cmd.Flags().GetBool("bodies")
	maxTokens, _ := cmd.Flags().GetInt("max-tokens")
	wholeFile, _ := cmd.Flags().GetBool("whole-file")

	if format != "go" && format != "md" {
		return DiffArgs{}, fmt.Errorf(
			"invalid --format %q (want: go|md)", format)
	}

	return DiffArgs{
		FilePath:   path,
		Ref:        ref,
		OutputFile: output,
		Format:     format,
		Bodies:     bodies,
		MaxTokens:  maxTokens,
		WholeFile:  wholeFile,
	}, nil
}

func diffAction(ctx context.Context, args DiffArgs) (cli.Output, error) {
	changed, err := changedGoFiles(ctx, args.Ref, args.FilePath)
	if err != nil {
		return cli.Output{}, err
	}
	if len(changed) == 0 {
		return cli.Output{
			Text:     fmt.Sprintf("No Go files changed since %s.\n", args.Ref),
			ExitCode: 0,
		}, nil
	}

	// By default we filter symbols to those whose line range actually
	// overlaps a changed hunk. --whole-file disables this — useful when
	// the user wants the full surrounding context for every modified
	// file (e.g. heavily refactored PRs where line-level filtering hides
	// related context).
	var hunks map[string][]hunkRange
	if !args.WholeFile {
		hunks, err = changedHunks(ctx, args.Ref, args.FilePath)
		if err != nil {
			// Surface the error but fall through to whole-file mode
			// rather than failing — better to over-include than to
			// stop the user dead because git changed its diff format.
			fmt.Fprintf(os.Stderr,
				"warning: --unified=0 hunk parse failed (%v); falling back to whole-file mode\n", err)
			hunks = nil
			args.WholeFile = true
		}
	}

	symbols, pkgName, err := collectSymbolsFromFiles(changed, args.Bodies, hunks)
	if err != nil {
		return cli.Output{}, fmt.Errorf("collect symbols: %w", err)
	}

	out := renderDiffOutput(pkgName, args.Ref, changed, symbols, args.Bodies)

	if args.MaxTokens > 0 {
		var truncated bool
		out, truncated = extract.TruncateSymbols(out, args.MaxTokens)
		if truncated {
			fmt.Fprintf(os.Stderr,
				"notice: diff truncated to fit --max-tokens %d\n", args.MaxTokens)
		}
	}
	if args.Format == "md" {
		title := fmt.Sprintf("Changes since %s", args.Ref)
		out = renderAsMarkdown(title, out)
	}

	if args.OutputFile != "" {
		if err := os.WriteFile(args.OutputFile, []byte(out), 0644); err != nil {
			return cli.Output{}, fmt.Errorf("write %s: %w", args.OutputFile, err)
		}
		return cli.Output{
			Text:     fmt.Sprintf("✓ Written to %s\n", args.OutputFile),
			ExitCode: 0,
		}, nil
	}

	return cli.Output{Text: out, ExitCode: 0}, nil
}

// changedGoFiles asks git which files differ between ref and HEAD,
// then narrows the result to .go files inside the requested path.
// We use --name-only on the diff so we don't have to parse the patch.
func changedGoFiles(ctx context.Context, ref, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", ref, "HEAD")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		// git's stderr is often the most informative thing in this
		// failure mode (unknown ref, not a git repo, etc).
		return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(output)))
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".go") {
			continue
		}
		// git diff returns paths relative to the repo root, which is
		// what cmd.Dir was set to. Resolve to a real path under root.
		full := filepath.Join(root, line)
		if _, err := os.Stat(full); err == nil {
			files = append(files, full)
		}
	}
	sort.Strings(files)
	return files, nil
}

// collectSymbolsFromFiles parses each given file and yields the same
// dumpedSymbol shape that `dump` uses, so renderDump can format the
// result. Internally it goes through the same extract.DiscoverAllSymbols
// path as `dump` — only the file selection differs (an explicit list
// from `git diff` rather than a directory walk).
//
// When hunks is non-nil, symbols are further filtered to those whose
// line range overlaps a changed hunk in their file. A nil hunks map
// preserves whole-file behavior (the default for non-diff callers,
// and what --whole-file gives in the diff verb).
func collectSymbolsFromFiles(paths []string, includeBodies bool, hunks map[string][]hunkRange) ([]dumpedSymbol, string, error) {
	if len(paths) == 0 {
		return nil, "", nil
	}

	fset := token.NewFileSet()
	pkgNode, err := extract.ExtractPackageFromPaths(fset, paths)
	if err != nil {
		// Even on partial parse failure, ExtractPackageFromPaths returns
		// what it could — we surface the error but also use whatever
		// PackageNode came back, so the user sees changes from files
		// that did parse.
		if pkgNode.Name == "" {
			return nil, "", err
		}
	}

	all := extract.DiscoverAllSymbols(pkgNode)
	dumped := make([]dumpedSymbol, 0, len(all))
	for _, s := range all {
		if s.Decl == nil {
			continue
		}
		startPos := fset.Position(s.Decl.Pos())
		endPos := fset.Position(s.Decl.End())

		// Hunk-based filtering: skip symbols whose line range doesn't
		// touch any changed hunk in the same file. fset.Position gives
		// us absolute filenames; the hunks map is keyed by absolute
		// path too (changedHunks resolves via filepath.Join with root).
		if hunks != nil {
			fileHunks := hunks[startPos.Filename]
			if len(fileHunks) == 0 {
				continue
			}
			if !rangeOverlaps(startPos.Line, endPos.Line, fileHunks) {
				continue
			}
		}

		kind := normalizeDumpKind(s.Kind)
		ds := dumpedSymbol{
			Kind:     kind,
			Name:     s.Name,
			Receiver: s.Receiver,
			File:     filepath.Base(startPos.Filename),
			Line:     startPos.Line,
		}
		ds.Source = renderSymbolSource(fset, s, includeBodies)
		ds.Doc = extract.SymbolDoc(s.Decl)
		dumped = append(dumped, ds)
	}

	sort.Slice(dumped, func(i, j int) bool {
		if dumped[i].File != dumped[j].File {
			return dumped[i].File < dumped[j].File
		}
		return dumped[i].Line < dumped[j].Line
	})

	return dumped, pkgNode.Name, nil
}

func renderDiffOutput(pkgName, ref string, files []string, symbols []dumpedSymbol, bodies bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// pureast diff: package %s — changes since %s\n", pkgName, ref)
	fmt.Fprintf(&b, "// %d changed file(s), %d symbol(s)", len(files), len(symbols))
	if !bodies {
		b.WriteString(" (signatures only)")
	}
	b.WriteString("\n\n")

	for _, f := range files {
		fmt.Fprintf(&b, "// changed: %s\n", f)
	}
	b.WriteString("\n")

	// Group by kind, same convention as `dump`, so the output is
	// recognizable to anyone familiar with that command.
	groups := map[string][]dumpedSymbol{}
	order := []string{"struct", "interface", "type", "func", "method", "const", "var"}
	for _, s := range symbols {
		groups[s.Kind] = append(groups[s.Kind], s)
	}
	headings := map[string]string{
		"struct":    "// === structs ===",
		"interface": "// === interfaces ===",
		"type":      "// === type aliases ===",
		"func":      "// === functions ===",
		"method":    "// === methods ===",
		"const":     "// === constants ===",
		"var":       "// === variables ===",
	}
	for _, kind := range order {
		ss := groups[kind]
		if len(ss) == 0 {
			continue
		}
		b.WriteString(headings[kind])
		b.WriteString("\n\n")
		for _, s := range ss {
			docEmittedBySource := bodies && (s.Kind == "func" || s.Kind == "method")
			if s.Doc != "" && !docEmittedBySource {
				for _, line := range strings.Split(strings.TrimRight(s.Doc, "\n"), "\n") {
					b.WriteString("// ")
					b.WriteString(line)
					b.WriteString("\n")
				}
			}
			b.WriteString(s.Source)
			b.WriteString("\n\n")
		}
	}

	return b.String()
}
