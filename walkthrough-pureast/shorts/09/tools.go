package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"strings"

	"github.com/Pure-Company/pureast/pkg/analyze"
	astpkg "github.com/Pure-Company/pureast/pkg/ast"
	"github.com/Pure-Company/pureast/pkg/codegen"
	"github.com/Pure-Company/pureast/pkg/extract"
	"github.com/Pure-Company/purekernels/pkg/functor"
	"github.com/Pure-Company/purekernels/pkg/monoid"
	"github.com/Pure-Company/purekernels/pkg/result"
)

// ToolExecutor executes pureast tools using applicative kernels
type ToolExecutor struct {
	workers int
}

// NewToolExecutor creates a tool executor
func NewToolExecutor(workers int) *ToolExecutor {
	return &ToolExecutor{workers: workers}
}

// SearchSymbolsHandler searches for symbols using fuzzy matching.
//
// The fuzzy parameter is preserved for backward compatibility with
// existing MCP clients: fuzzy=true allows subsequence/initials matches
// (e.g. "Hndl" → "Handler"), fuzzy=false restricts to substring matches
// only (score >= 400 in the underlying scoring).
func (te *ToolExecutor) SearchSymbolsHandler() Handler {
	return func(ctx context.Context, req MCPRequest) functor.Concurrent[MCPResponse] {
		responseMonoid := NewResponseMonoid()

		return functor.NewConcurrent(
			responseMonoid,
			func() MCPResponse {
				var params struct {
					Name      string `json:"name"`
					Arguments struct {
						Pattern    string `json:"pattern"`
						Path       string `json:"path"`
						Fuzzy      bool   `json:"fuzzy"`
						Kind       string `json:"kind,omitempty"`
						MaxResults int    `json:"maxResults,omitempty"`
					} `json:"arguments"`
				}

				if err := json.Unmarshal(req.Params, &params); err != nil {
					return ErrorResponse(req.ID, InvalidParams, "Invalid parameters")
				}

				fset := token.NewFileSet()
				pkgResult := loadPackage(fset, params.Arguments.Path, te.workers)
				if !pkgResult.IsOk() {
					return ErrorResponse(req.ID, InternalError, pkgResult.Error().Error())
				}
				pkgNode := pkgResult.Unwrap()

				maxResults := params.Arguments.MaxResults
				if maxResults <= 0 || maxResults > 100 {
					maxResults = 20
				}

				symbols := extract.DiscoverAllSymbols(pkgNode)
				matches := extract.FuzzySearch(
					symbols,
					params.Arguments.Pattern,
					params.Arguments.Kind,
					maxResults,
				)

				// fuzzy=false restricts to substring-or-better matches.
				// FuzzySearch's scoring guarantees: 1000 = exact, 800 =
				// prefix, 400-600 = contains, 100-300 = subsequence,
				// 50 = initials. The 400 cutoff drops subsequence and
				// initials matches.
				if !params.Arguments.Fuzzy {
					filtered := matches[:0]
					for _, m := range matches {
						if m.Score >= 400 {
							filtered = append(filtered, m)
						}
					}
					matches = filtered
				}

				return MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": formatSearchResults(matches, pkgNode.Name),
							},
						},
					},
				}
			},
		)
	}
}

// ExtractSymbolHandler extracts a symbol with dependencies
func (te *ToolExecutor) ExtractSymbolHandler() Handler {
	return func(ctx context.Context, req MCPRequest) functor.Concurrent[MCPResponse] {
		responseMonoid := NewResponseMonoid()

		return functor.NewConcurrent(
			responseMonoid,
			func() MCPResponse {
				// Parse parameters
				var params struct {
					Name      string `json:"name"`
					Arguments struct {
						Symbol  string `json:"symbol"`
						Path    string `json:"path"`
						Minimal bool   `json:"minimal"`
					} `json:"arguments"`
				}

				if err := json.Unmarshal(req.Params, &params); err != nil {
					return ErrorResponse(req.ID, InvalidParams, "Invalid parameters")
				}

				// Load package
				fset := token.NewFileSet()
				pkgResult := loadPackage(fset, params.Arguments.Path, te.workers)

				if !pkgResult.IsOk() {
					return ErrorResponse(req.ID, InternalError, pkgResult.Error().Error())
				}

				pkgNode := pkgResult.Unwrap()

				// Build dependency graph
				declMap := extract.BuildPackageDeclMap(pkgNode)
				graph := analyze.NewDependencyGraph(declMap)

				// Resolve dependencies with associated code
				deps := graph.ResolveWithAssociatedCode(params.Arguments.Symbol)

				// Generate code
				gen := codegen.NewGenerator(fset)
				code, err := gen.GenerateMinimal(
					pkgNode.Name,
					params.Arguments.Symbol,
					declMap,
					deps,
				)

				if err != nil {
					return ErrorResponse(req.ID, InternalError, err.Error())
				}

				// Return CallToolResult format
				return MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": code,
							},
						},
					},
				}
			},
		)
	}
}

// ListSymbolsHandler lists all symbols in a package
func (te *ToolExecutor) ListSymbolsHandler() Handler {
	return func(ctx context.Context, req MCPRequest) functor.Concurrent[MCPResponse] {
		responseMonoid := NewResponseMonoid()

		return functor.NewConcurrent(
			responseMonoid,
			func() MCPResponse {
				// Parse parameters
				var params struct {
					Name      string `json:"name"`
					Arguments struct {
						Path        string `json:"path"`
						GroupByKind bool   `json:"groupByKind"`
					} `json:"arguments"`
				}

				if err := json.Unmarshal(req.Params, &params); err != nil {
					return ErrorResponse(req.ID, InvalidParams, "Invalid parameters")
				}

				// Load package
				fset := token.NewFileSet()
				pkgResult := loadPackage(fset, params.Arguments.Path, te.workers)

				if !pkgResult.IsOk() {
					return ErrorResponse(req.ID, InternalError, pkgResult.Error().Error())
				}

				pkgNode := pkgResult.Unwrap()

				// Discover symbols
				symbols := extract.DiscoverAllSymbols(pkgNode)

				// Format output
				text := extract.FormatSymbolList(symbols, params.Arguments.GroupByKind)

				// Return CallToolResult format
				return MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": text,
							},
						},
					},
				}
			},
		)
	}
}

// ExtractTypesHandler extracts type definitions
func (te *ToolExecutor) ExtractTypesHandler() Handler {
	return func(ctx context.Context, req MCPRequest) functor.Concurrent[MCPResponse] {
		responseMonoid := NewResponseMonoid()

		return functor.NewConcurrent(
			responseMonoid,
			func() MCPResponse {
				// Parse parameters
				var params struct {
					Name      string `json:"name"`
					Arguments struct {
						Path           string `json:"path"`
						StructsOnly    bool   `json:"structsOnly"`
						InterfacesOnly bool   `json:"interfacesOnly"`
					} `json:"arguments"`
				}

				if err := json.Unmarshal(req.Params, &params); err != nil {
					return ErrorResponse(req.ID, InvalidParams, "Invalid parameters")
				}

				// Load package
				fset := token.NewFileSet()
				pkgResult := loadPackage(fset, params.Arguments.Path, te.workers)

				if !pkgResult.IsOk() {
					return ErrorResponse(req.ID, InternalError, pkgResult.Error().Error())
				}

				pkgNode := pkgResult.Unwrap()

				// Extract types
				var types []extract.TypeDeclaration
				if params.Arguments.StructsOnly {
					types = extract.ExtractAllStructs(pkgNode)
				} else if params.Arguments.InterfacesOnly {
					types = extract.ExtractAllInterfaces(pkgNode)
				} else {
					types = extract.ExtractAllStructsAndInterfaces(pkgNode)
				}

				// Generate code
				gen := codegen.NewGenerator(fset)
				code, err := gen.GenerateTypesOnly(
					pkgNode.Name,
					types,
					pkgNode.Deps.Imports.ToSlice(),
				)

				if err != nil {
					return ErrorResponse(req.ID, InternalError, err.Error())
				}

				// Return CallToolResult format
				return MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": code,
							},
						},
					},
				}
			},
		)
	}
}

// ShowDependenciesHandler shows dependencies for a symbol
func (te *ToolExecutor) ShowDependenciesHandler() Handler {
	return func(ctx context.Context, req MCPRequest) functor.Concurrent[MCPResponse] {
		responseMonoid := NewResponseMonoid()

		return functor.NewConcurrent(
			responseMonoid,
			func() MCPResponse {
				// Parse parameters
				var params struct {
					Name      string `json:"name"`
					Arguments struct {
						Symbol string `json:"symbol"`
						Path   string `json:"path"`
					} `json:"arguments"`
				}

				if err := json.Unmarshal(req.Params, &params); err != nil {
					return ErrorResponse(req.ID, InvalidParams, "Invalid parameters")
				}

				// Load package
				fset := token.NewFileSet()
				pkgResult := loadPackage(fset, params.Arguments.Path, te.workers)

				if !pkgResult.IsOk() {
					return ErrorResponse(req.ID, InternalError, pkgResult.Error().Error())
				}

				pkgNode := pkgResult.Unwrap()

				// Build dependency graph
				declMap := extract.BuildPackageDeclMap(pkgNode)
				graph := analyze.NewDependencyGraph(declMap)

				// Resolve dependencies
				deps := graph.ResolveTransitive(params.Arguments.Symbol)

				// Format output
				text := formatDependencies(params.Arguments.Symbol, deps)

				// Return CallToolResult format
				return MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": text,
							},
						},
					},
				}
			},
		)
	}
}

// Helper functions

func loadPackage(fset *token.FileSet, path string, workers int) result.Result[astpkg.PackageNode] {
	pkgNode, err := extract.ExtractDirectoryConcurrent(fset, path, true, workers)
	if err != nil {
		return result.Err[astpkg.PackageNode](err)
	}
	return result.Ok(pkgNode)
}

func formatSearchResults(matches []extract.Match, pkgName string) string {
	if len(matches) == 0 {
		return "No symbols found."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d symbols:\n\n", len(matches))
	for _, m := range matches {
		fmt.Fprintf(&b, "- %s (%s) [score: %d] in package %s\n",
			m.Symbol.Name, m.Symbol.Kind, m.Score, pkgName)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatDependencies(symbol string, deps astpkg.Dependencies) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Dependencies for %s:\n", symbol))

	if deps.Types.Size() > 0 {
		parts = append(parts, fmt.Sprintf("\nTypes (%d):", deps.Types.Size()))
		for _, name := range deps.Types.ToSlice() {
			parts = append(parts, "  - "+name)
		}
	}

	if deps.Functions.Size() > 0 {
		parts = append(parts, fmt.Sprintf("\nFunctions (%d):", deps.Functions.Size()))
		for _, name := range deps.Functions.ToSlice() {
			parts = append(parts, "  - "+name)
		}
	}

	if deps.Imports.Size() > 0 {
		parts = append(parts, fmt.Sprintf("\nImports (%d):", deps.Imports.Size()))
		for _, name := range deps.Imports.ToSlice() {
			parts = append(parts, "  - "+name)
		}
	}

	return monoid.Reduce(monoid.NewStringJoinMonoid("\n"), parts)
}
