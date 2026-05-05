// cmd/pureast/commands/extract.go - Cleaned up
package commands

import (
	"context"
	"fmt"
	"go/token"
	"os"

	"github.com/Pure-Company/pureast/pkg/analyze"
	astpkg "github.com/Pure-Company/pureast/pkg/ast"
	"github.com/Pure-Company/pureast/pkg/cli"
	"github.com/Pure-Company/pureast/pkg/codegen"
	"github.com/Pure-Company/pureast/pkg/extract"
	"github.com/spf13/cobra"
)

type ExtractArgs struct {
	FilePath   string
	Symbol     string
	OutputFile string
	Format     string // go|md
	Minimal    bool
	Workers    int
	MaxTokens  int
}

func NewExtractCommand() *cobra.Command {
	cmd := cli.NewCommand[ExtractArgs]("extract").
		Short("Extract a symbol with all dependencies").
		Long(`Extract a Go symbol with all its dependencies and associated code.

By default, includes:
  - Type definition
  - All dependencies (transitive)
  - Constructors (NewX functions)
  - Methods

Examples:
  pureast extract User ./pkg
  pureast extract Profile ./pkg --minimal
  pureast extract UserService ./pkg -o service.go
  pureast extract User ./pkg --format md
  pureast extract User ./pkg --max-tokens 2000`).
		ParseArgs(parseExtractArgs).
		Action(extractAction).
		Build()

	cmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	cmd.Flags().String("format", "go", "Output format: go|md")
	cmd.Flags().Bool("minimal", false, "Extract minimal dependencies only")
	cmd.Flags().IntP("workers", "w", 0, "Number of workers (0 = auto)")
	cmd.Flags().Int("max-tokens", 0, "Truncate output to fit token budget (0 = unbounded)")

	// Back-compat: --file kept as deprecated alias. The positional
	// PATH is canonical; --file warns and is removed in a future release.
	cmd.Flags().StringP("file", "f", "", "[deprecated] use positional PATH")

	return cmd
}

func parseExtractArgs(cmd *cobra.Command, args []string) (ExtractArgs, error) {
	if len(args) < 1 {
		return ExtractArgs{}, fmt.Errorf("requires SYMBOL [PATH]")
	}
	if len(args) > 2 {
		return ExtractArgs{}, fmt.Errorf("expected SYMBOL [PATH], got %d args", len(args))
	}

	path, err := resolvePathFromTail(cmd, args[1:])
	if err != nil {
		return ExtractArgs{}, err
	}

	output, _ := cmd.Flags().GetString("output")
	format, _ := cmd.Flags().GetString("format")
	minimal, _ := cmd.Flags().GetBool("minimal")
	workers, _ := cmd.Flags().GetInt("workers")
	maxTokens, _ := cmd.Flags().GetInt("max-tokens")

	if format != "go" && format != "md" {
		return ExtractArgs{}, fmt.Errorf(
			"invalid --format %q (want: go|md)", format)
	}

	return ExtractArgs{
		FilePath:   path,
		Symbol:     args[0],
		OutputFile: output,
		Format:     format,
		Minimal:    minimal,
		Workers:    workers,
		MaxTokens:  maxTokens,
	}, nil
}

func extractAction(ctx context.Context, args ExtractArgs) (cli.Output, error) {
	fset := token.NewFileSet()
	pkgNode, err := extract.ExtractDirectoryConcurrent(fset, args.FilePath, true, args.Workers)
	if err != nil {
		return cli.Output{}, fmt.Errorf("extract %s: %w", args.FilePath, err)
	}

	declMap := extract.BuildPackageDeclMap(pkgNode)
	graph := analyze.NewDependencyGraph(declMap)

	// Choose query strategy
	var deps astpkg.Dependencies
	if args.Minimal {
		deps = graph.MinimalDependencies(args.Symbol)
	} else {
		deps = graph.ResolveWithAssociatedCode(args.Symbol)
	}

	gen := codegen.NewGenerator(fset)
	code, err := gen.GenerateMinimal(pkgNode.Name, args.Symbol, declMap, deps)
	if err != nil {
		return cli.Output{}, fmt.Errorf("generate %s: %w", args.Symbol, err)
	}

	// Token budget applied before format wrapping so a markdown fence
	// always closes properly. Symbol-aware truncation drops trailing
	// whole declarations rather than slicing through one — the result
	// stays compilable Go.
	if args.MaxTokens > 0 {
		var truncated bool
		code, truncated = extract.TruncateSymbols(code, args.MaxTokens)
		if truncated {
			fmt.Fprintf(os.Stderr,
				"notice: extract truncated to fit --max-tokens %d\n", args.MaxTokens)
		}
	}
	if args.Format == "md" {
		title := fmt.Sprintf("%s.%s", pkgNode.Name, args.Symbol)
		code = renderAsMarkdown(title, code)
	}

	if args.OutputFile != "" {
		if err := os.WriteFile(args.OutputFile, []byte(code), 0644); err != nil {
			return cli.Output{}, fmt.Errorf("write %s: %w", args.OutputFile, err)
		}
		return cli.Output{
			Text:     fmt.Sprintf("✅ Written to %s\n", args.OutputFile),
			ExitCode: 0,
		}, nil
	}

	return cli.Output{Text: code, ExitCode: 0}, nil
}
