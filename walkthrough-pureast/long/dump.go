// cmd/pureast/commands/dump.go
//
// Compact, LLM-friendly dump of every symbol in a package.
//
// Unlike `types` (only type declarations) or `extract` (one symbol + its
// transitive deps), `dump` walks the whole package and emits every top-level
// symbol the parser can see: structs, interfaces, type aliases, functions,
// methods, consts, vars.
//
// The default output is signature-only — bodies are stripped. This is the
// single biggest token-compression win: a 5000-line package collapses to
// a few hundred lines of "what exists, what it returns, what it satisfies."
// Use --bodies if you need the implementations too.
package commands

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Pure-Company/pureast/pkg/cli"
	"github.com/Pure-Company/pureast/pkg/extract"
	"github.com/spf13/cobra"
)

type DumpArgs struct {
	FilePath     string
	OutputFile   string
	Kind         string // all|type|struct|interface|func|method|const|var
	Format       string // go|md
	Bodies       bool   // include function bodies (default: signatures only)
	ExportedOnly bool
	IncludeTests bool
	IncludeDocs  bool
	MaxTokens    int // 0 = unbounded
}

type dumpedSymbol struct {
	Kind     string // struct, interface, type, func, method, const, var
	Name     string
	Receiver string // for methods
	Doc      string
	Source   string // signature or full text
	File     string
	Line     int
}

func NewDumpCommand() *cobra.Command {
	cmd := cli.NewCommand[DumpArgs]("dump").
		Short("Dump every symbol in a package (LLM context)").
		Long(`Dump all top-level symbols — types, functions, methods, consts, vars —
in a compact form suitable for feeding to an LLM as context.

By default, function bodies are stripped (signatures only). This typically
gives 5–20× compression versus pasting raw source files.

Examples:
  pureast dump ./pkg                        # everything, signatures only
  pureast dump ./pkg --bodies               # include implementations
  pureast dump ./pkg --kind func            # only functions
  pureast dump ./pkg --exported             # only exported symbols
  pureast dump ./pkg --format md            # markdown for LLM
  pureast dump ./pkg --max-tokens 4000      # fit a token budget
  pureast dump ./pkg -o context.txt`).
		ParseArgs(parseDumpArgs).
		Action(dumpAction).
		Build()

	cmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	cmd.Flags().String("kind", "all", "Filter: all|type|struct|interface|func|method|const|var")
	cmd.Flags().String("format", "go", "Output format: go|md")
	cmd.Flags().Bool("bodies", false, "Include function bodies (default: signatures only)")
	cmd.Flags().Bool("exported", false, "Only exported symbols")
	cmd.Flags().Bool("include-tests", false, "Include _test.go files")
	cmd.Flags().Bool("no-docs", false, "Strip doc comments")
	cmd.Flags().Int("max-tokens", 0, "Truncate output to fit token budget (0 = unbounded)")

	// --file kept as a back-compat alias; positional path is canonical
	cmd.Flags().StringP("file", "f", "", "[deprecated] use positional PATH")

	return cmd
}

func parseDumpArgs(cmd *cobra.Command, args []string) (DumpArgs, error) {
	path, err := resolvePath(cmd, args)
	if err != nil {
		return DumpArgs{}, err
	}

	output, _ := cmd.Flags().GetString("output")
	kind, _ := cmd.Flags().GetString("kind")
	format, _ := cmd.Flags().GetString("format")
	bodies, _ := cmd.Flags().GetBool("bodies")
	exported, _ := cmd.Flags().GetBool("exported")
	tests, _ := cmd.Flags().GetBool("include-tests")
	noDocs, _ := cmd.Flags().GetBool("no-docs")
	maxTokens, _ := cmd.Flags().GetInt("max-tokens")

	if !validDumpKind(kind) {
		return DumpArgs{}, fmt.Errorf(
			"invalid --kind %q (want: all|type|struct|interface|func|method|const|var)", kind)
	}
	if format != "go" && format != "md" {
		return DumpArgs{}, fmt.Errorf(
			"invalid --format %q (want: go|md)", format)
	}

	return DumpArgs{
		FilePath:     path,
		OutputFile:   output,
		Kind:         kind,
		Format:       format,
		Bodies:       bodies,
		ExportedOnly: exported,
		IncludeTests: tests,
		IncludeDocs:  !noDocs,
		MaxTokens:    maxTokens,
	}, nil
}

func validDumpKind(k string) bool {
	switch k {
	case "all", "type", "struct", "interface", "func", "method", "const", "var":
		return true
	}
	return false
}

func dumpAction(ctx context.Context, args DumpArgs) (cli.Output, error) {
	symbols, pkgName, err := collectSymbols(args)
	if err != nil {
		return cli.Output{}, fmt.Errorf("collect symbols from %s: %w", args.FilePath, err)
	}

	out := renderDump(pkgName, symbols, args)

	// Token budget is applied to the final rendered text, not the
	// individual symbols. We use symbol-aware truncation so the result
	// is always syntactically complete Go: we drop trailing whole
	// declarations rather than slice through one. Inside a markdown
	// fence this matters — partial output produces invalid Go that
	// the LLM then has to repair.
	if args.MaxTokens > 0 {
		var truncated bool
		out, truncated = extract.TruncateSymbols(out, args.MaxTokens)
		if truncated {
			fmt.Fprintf(os.Stderr,
				"notice: dump truncated to fit --max-tokens %d\n", args.MaxTokens)
		}
	}

	// Markdown wrapping is applied last so the fence wraps whatever
	// fit inside the budget, including any truncation marker.
	if args.Format == "md" {
		title := fmt.Sprintf("Package %s", pkgName)
		out = renderAsMarkdown(title, out)
	}

	if args.OutputFile != "" {
		if err := os.WriteFile(args.OutputFile, []byte(out), 0644); err != nil {
			return cli.Output{}, fmt.Errorf("write %s: %w", args.OutputFile, err)
		}
		return cli.Output{
			Text:     fmt.Sprintf("✓ Written %d symbols to %s\n", len(symbols), args.OutputFile),
			ExitCode: 0,
		}, nil
	}

	return cli.Output{Text: out, ExitCode: 0}, nil
}

// collectSymbols loads the package, discovers every top-level symbol
// via extract.DiscoverAllSymbols (the canonical walker), filters by
// the user's --kind / --exported / --include-tests flags, and returns
// dumpedSymbol records ready for rendering.
//
// This used to be a parallel AST walker that duplicated discovery
// logic. Now it's a thin adapter: the heavy lifting happens in
// pkg/extract, the per-symbol rendering happens here. The renderer
// functions (renderFuncDecl, renderTypeSpec, etc.) consume the
// SymbolInfo.Decl that DiscoverAllSymbols already populates.
func collectSymbols(args DumpArgs) ([]dumpedSymbol, string, error) {
	fset := token.NewFileSet()
	pkgNode, err := extract.ExtractDirectoryConcurrent(fset, args.FilePath, true, 0)
	if err != nil {
		return nil, "", err
	}

	all := extract.DiscoverAllSymbols(pkgNode)

	dumped := make([]dumpedSymbol, 0, len(all))
	for _, s := range all {
		// Map the canonical kind names ("function") to the dump verb's
		// shorter aliases ("func"). Keeping --kind func in the CLI is
		// nicer than --kind function; the discovery layer uses the
		// long form because that's the AST/Go vernacular.
		kind := normalizeDumpKind(s.Kind)
		if !kindAllowed(args.Kind, kind) {
			continue
		}
		if args.ExportedOnly && !ast.IsExported(s.Name) {
			continue
		}
		if !args.IncludeTests && declInTestFile(fset, s.Decl) {
			continue
		}

		ds := dumpedSymbol{
			Kind:     kind,
			Name:     s.Name,
			Receiver: s.Receiver,
		}
		if s.Decl != nil {
			pos := fset.Position(s.Decl.Pos())
			rel, relErr := filepath.Rel(args.FilePath, pos.Filename)
			if relErr != nil || strings.HasPrefix(rel, "..") {
				rel = pos.Filename
			}
			ds.File = rel
			ds.Line = pos.Line
		}
		ds.Source = renderSymbolSource(fset, s, args.Bodies)
		if args.IncludeDocs {
			ds.Doc = extract.SymbolDoc(s.Decl)
		}
		dumped = append(dumped, ds)
	}

	// Stable order: file, then line. Required for byte-identical
	// output across runs (prompt caching).
	sort.Slice(dumped, func(i, j int) bool {
		if dumped[i].File != dumped[j].File {
			return dumped[i].File < dumped[j].File
		}
		return dumped[i].Line < dumped[j].Line
	})

	return dumped, pkgNode.Name, nil
}

// normalizeDumpKind maps DiscoverAllSymbols' kind names to the CLI's
// --kind aliases. Keeping the user-facing "func" tighter than the
// AST-internal "function" is a small UX win; nobody types "function"
// at a CLI when "func" works.
func normalizeDumpKind(k string) string {
	switch k {
	case "function":
		return "func"
	}
	return k
}

// declInTestFile reports whether a declaration lives in a _test.go file.
// The discovery layer doesn't filter tests itself; we do that at the
// dump layer because list/types might want them in some contexts.
func declInTestFile(fset *token.FileSet, decl ast.Decl) bool {
	if decl == nil {
		return false
	}
	pos := fset.Position(decl.Pos())
	return strings.HasSuffix(pos.Filename, "_test.go")
}

// renderSymbolSource produces the per-symbol body text the dump verb
// emits. Delegates to pkg/extract — the actual rendering logic lives
// there now so MCP can use the same code. The local wrapper exists
// so the body switch (signature vs full source) reads naturally at
// the call site.
func renderSymbolSource(fset *token.FileSet, s extract.SymbolInfo, includeBody bool) string {
	if s.Decl == nil {
		// Defensive: should not happen, but emit something tractable
		// rather than panic on a malformed input.
		return s.Kind + " " + s.Name
	}
	if includeBody {
		return extract.RenderWithBody(fset, s)
	}
	return extract.RenderSignature(fset, s)
}

func kindAllowed(filter, kind string) bool {
	if filter == "all" {
		return true
	}
	if filter == kind {
		return true
	}
	// "type" matches struct/interface/alias
	if filter == "type" && (kind == "struct" || kind == "interface" || kind == "type") {
		return true
	}
	return false
}

func renderDump(pkgName string, symbols []dumpedSymbol, args DumpArgs) string {
	var b strings.Builder

	// Header — orientation for the LLM
	fmt.Fprintf(&b, "// pureast dump: package %s\n", pkgName)
	fmt.Fprintf(&b, "// %d symbols", len(symbols))
	if !args.Bodies {
		b.WriteString(" (signatures only)")
	}
	if args.ExportedOnly {
		b.WriteString(", exported only")
	}
	b.WriteString("\n\n")

	// Group by kind for readability — order: types, then funcs/methods, then values
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
			// printNode (used in --bodies mode for funcs/methods) emits
			// the doc comment itself; don't double-print it.
			docEmittedBySource := args.Bodies && (s.Kind == "func" || s.Kind == "method")
			if args.IncludeDocs && s.Doc != "" && !docEmittedBySource {
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

// resolvePath, resolvePathFromTail, estimateTokens, etc. live in helpers.go.
