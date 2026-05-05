package analyze

import (
	"go/ast"

	astpkg "github.com/Pure-Company/pureast/pkg/ast"
	"github.com/Pure-Company/purekernels/pkg/fold"
	"github.com/Pure-Company/purekernels/pkg/monoid"
)

// DependencyGraph represents the full dependency structure
type DependencyGraph struct {
	Decls map[string]astpkg.DeclNode
}

// NewDependencyGraph creates a graph from declaration map
func NewDependencyGraph(decls map[string]astpkg.DeclNode) DependencyGraph {
	return DependencyGraph{Decls: decls}
}

// ResolveTransitive computes transitive closure of dependencies
// Uses fixed-point iteration with monoid accumulation
func (g DependencyGraph) ResolveTransitive(targetName string) astpkg.Dependencies {
	visited := monoid.NewSetMonoid[string]()
	return g.resolveTransitiveRec(targetName, visited)
}

// resolveTransitiveRec - recursive helper with visited tracking
func (g DependencyGraph) resolveTransitiveRec(
	name string,
	visited monoid.SetMonoid[string],
) astpkg.Dependencies {
	// Already visited? Return empty (monoid identity)
	if visited.Contains(name) {
		return astpkg.NewDependencies()
	}

	// Mark as visited
	visited = visited.Insert(name)

	// Find declaration
	decl, ok := g.Decls[name]
	if !ok {
		return astpkg.NewDependencies()
	}

	// Start with immediate dependencies
	immediateDeps := decl.Deps

	// Resolve all type dependencies transitively
	typeDeps := g.resolveSetTransitive(immediateDeps.Types.ToSlice(), visited)

	// Resolve all function dependencies transitively
	funcDeps := g.resolveSetTransitive(immediateDeps.Functions.ToSlice(), visited)

	// Resolve all struct dependencies transitively
	structDeps := g.resolveSetTransitive(immediateDeps.Structs.ToSlice(), visited)

	// Resolve all interface dependencies transitively
	ifaceDeps := g.resolveSetTransitive(immediateDeps.Interfaces.ToSlice(), visited)

	// Combine all using monoid
	depMonoid := astpkg.NewDependencyMonoid()
	return monoid.Reduce(
		depMonoid,
		[]astpkg.Dependencies{
			immediateDeps,
			typeDeps,
			funcDeps,
			structDeps,
			ifaceDeps,
		},
	)
}

// resolveSetTransitive resolves a set of names transitively
func (g DependencyGraph) resolveSetTransitive(
	names []string,
	visited monoid.SetMonoid[string],
) astpkg.Dependencies {
	// Map each name to its transitive dependencies
	depsList := fold.Map(
		func(name string) astpkg.Dependencies {
			return g.resolveTransitiveRec(name, visited)
		},
		names,
	)

	// Combine all using monoid reduction
	depMonoid := astpkg.NewDependencyMonoid()
	return monoid.Reduce(depMonoid, depsList)
}

// ResolveMultiple resolves dependencies for multiple targets
func (g DependencyGraph) ResolveMultiple(targets []string) map[string]astpkg.Dependencies {
	result := make(map[string]astpkg.Dependencies)

	for _, target := range targets {
		result[target] = g.ResolveTransitive(target)
	}

	return result
}

// MinimalDependencies computes minimal set needed for target
// Removes dependencies that are not reachable
func (g DependencyGraph) MinimalDependencies(targetName string) astpkg.Dependencies {
	allDeps := g.ResolveTransitive(targetName)

	// Filter to only include declarations that exist in our graph
	filteredTypes := g.filterExisting(allDeps.Types.ToSlice())
	filteredFuncs := g.filterExisting(allDeps.Functions.ToSlice())
	filteredStructs := g.filterExisting(allDeps.Structs.ToSlice())
	filteredIfaces := g.filterExisting(allDeps.Interfaces.ToSlice())

	return astpkg.Dependencies{
		Types:      monoid.FromSlice(filteredTypes),
		Functions:  monoid.FromSlice(filteredFuncs),
		Structs:    monoid.FromSlice(filteredStructs),
		Interfaces: monoid.FromSlice(filteredIfaces),
		Imports:    allDeps.Imports,
		Constants:  allDeps.Constants,
		Variables:  allDeps.Variables,
	}
}

// filterExisting keeps only names that exist in declaration map
func (g DependencyGraph) filterExisting(names []string) []string {
	return fold.Filter(
		func(name string) bool {
			_, ok := g.Decls[name]
			return ok
		},
		names,
	)
}

// DependencyOrder computes topological ordering of dependencies
// Returns declarations in order such that dependencies come before dependents
func (g DependencyGraph) DependencyOrder(targetName string) []string {
	//deps := g.ResolveTransitive(targetName)
	visited := monoid.NewSetMonoid[string]()
	order := []string{}

	// Helper for depth-first traversal
	var visit func(string)
	visit = func(name string) {
		if visited.Contains(name) {
			return
		}
		visited = visited.Insert(name)

		// Visit dependencies first
		if decl, ok := g.Decls[name]; ok {
			// Visit type dependencies
			for _, typeName := range decl.Deps.Types.ToSlice() {
				visit(typeName)
			}
			// Visit function dependencies
			for _, funcName := range decl.Deps.Functions.ToSlice() {
				visit(funcName)
			}
		}

		// Add to order after dependencies
		order = append(order, name)
	}

	// Start with target
	visit(targetName)

	return order
}

// CircularDependencies detects circular dependencies
func (g DependencyGraph) CircularDependencies(targetName string) [][]string {
	cycles := [][]string{}
	visited := monoid.NewSetMonoid[string]()
	recStack := monoid.NewSetMonoid[string]()

	var detectCycle func(string, []string) bool
	detectCycle = func(name string, path []string) bool {
		if recStack.Contains(name) {
			// Found cycle - extract it from path
			cycleStart := -1
			for i, p := range path {
				if p == name {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path)-cycleStart)
				copy(cycle, path[cycleStart:])
				cycles = append(cycles, cycle)
			}
			return true
		}

		if visited.Contains(name) {
			return false
		}

		visited = visited.Insert(name)
		recStack = recStack.Insert(name)
		path = append(path, name)

		decl, ok := g.Decls[name]
		if ok {
			// Check type dependencies
			for _, dep := range decl.Deps.Types.ToSlice() {
				if detectCycle(dep, path) {
					return true
				}
			}
			// Check function dependencies
			for _, dep := range decl.Deps.Functions.ToSlice() {
				if detectCycle(dep, path) {
					return true
				}
			}
		}

		recStack = monoid.NewSetMonoid[string]() // Remove from stack
		for _, p := range path[:len(path)-1] {
			recStack = recStack.Insert(p)
		}

		return false
	}

	detectCycle(targetName, []string{})
	return cycles
}

// UnusedDeclarations finds declarations not reachable from entry points
func (g DependencyGraph) UnusedDeclarations(entryPoints []string) []string {
	// Collect all reachable declarations
	reachable := monoid.NewSetMonoid[string]()

	for _, entry := range entryPoints {
		deps := g.ResolveTransitive(entry)
		reachable = reachable.Insert(entry)

		// Add all dependencies
		for _, name := range deps.Types.ToSlice() {
			reachable = reachable.Insert(name)
		}
		for _, name := range deps.Functions.ToSlice() {
			reachable = reachable.Insert(name)
		}
		for _, name := range deps.Structs.ToSlice() {
			reachable = reachable.Insert(name)
		}
		for _, name := range deps.Interfaces.ToSlice() {
			reachable = reachable.Insert(name)
		}
	}

	// Find unused declarations
	unused := []string{}
	for name := range g.Decls {
		if !reachable.Contains(name) {
			unused = append(unused, name)
		}
	}

	return unused
}

// DependencyStats computes statistics about dependencies
type DependencyStats struct {
	TotalTypes      int
	TotalFunctions  int
	TotalStructs    int
	TotalInterfaces int
	TotalImports    int
	MaxDepth        int
}

// ComputeStats computes dependency statistics for target
func (g DependencyGraph) ComputeStats(targetName string) DependencyStats {
	deps := g.ResolveTransitive(targetName)

	return DependencyStats{
		TotalTypes:      deps.Types.Size(),
		TotalFunctions:  deps.Functions.Size(),
		TotalStructs:    deps.Structs.Size(),
		TotalInterfaces: deps.Interfaces.Size(),
		TotalImports:    deps.Imports.Size(),
		MaxDepth:        g.computeMaxDepth(targetName, monoid.NewSetMonoid[string]()),
	}
}

// computeMaxDepth computes maximum dependency depth
func (g DependencyGraph) computeMaxDepth(
	name string,
	visited monoid.SetMonoid[string],
) int {
	if visited.Contains(name) {
		return 0
	}

	visited = visited.Insert(name)

	decl, ok := g.Decls[name]
	if !ok {
		return 0
	}

	maxDepth := 0

	// Check all dependencies
	allDeps := append(
		decl.Deps.Types.ToSlice(),
		decl.Deps.Functions.ToSlice()...,
	)

	for _, dep := range allDeps {
		depth := 1 + g.computeMaxDepth(dep, visited)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// SubgraphFor extracts subgraph containing only target and its dependencies
func (g DependencyGraph) SubgraphFor(targetName string) DependencyGraph {
	deps := g.ResolveTransitive(targetName)

	// Collect all relevant names
	relevantNames := monoid.NewSetMonoid[string]()
	relevantNames = relevantNames.Insert(targetName)

	for _, name := range deps.Types.ToSlice() {
		relevantNames = relevantNames.Insert(name)
	}
	for _, name := range deps.Functions.ToSlice() {
		relevantNames = relevantNames.Insert(name)
	}

	// Filter declarations
	subDecls := make(map[string]astpkg.DeclNode)
	for name := range g.Decls {
		if relevantNames.Contains(name) {
			subDecls[name] = g.Decls[name]
		}
	}

	return NewDependencyGraph(subDecls)
}

// FindFunctionsReturning finds all functions that return the given type
func (g DependencyGraph) FindFunctionsReturning(typeName string) []string {
	functions := []string{}

	for name, decl := range g.Decls {
		// Check if it's a function declaration
		if funcDecl, ok := decl.Decl.(*ast.FuncDecl); ok {
			// Skip methods (they have receivers)
			if funcDecl.Recv != nil {
				continue
			}

			// Check if it returns the type
			if funcReturnsType(funcDecl, typeName) {
				functions = append(functions, name)
			}
		}
	}

	return functions
}

// FindMethodsForType finds all methods on a given type
func (g DependencyGraph) FindMethodsForType(typeName string) []string {
	methods := []string{}

	for name, decl := range g.Decls {
		// Check if it's a method (function with receiver)
		if funcDecl, ok := decl.Decl.(*ast.FuncDecl); ok {
			if funcDecl.Recv != nil {
				recvType := extractReceiverTypeName(funcDecl.Recv)
				if recvType == typeName {
					methods = append(methods, name)
				}
			}
		}
	}

	return methods
}

// funcReturnsType checks if function returns the given type
func funcReturnsType(funcDecl *ast.FuncDecl, typeName string) bool {
	if funcDecl.Type.Results == nil {
		return false
	}

	for _, field := range funcDecl.Type.Results.List {
		if matchesType(field.Type, typeName) {
			return true
		}
	}

	return false
}

// matchesType checks if an expression matches a type name
func matchesType(expr ast.Expr, typeName string) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == typeName
	case *ast.StarExpr:
		return matchesType(t.X, typeName)
	}
	return false
}

// extractReceiverTypeName gets the receiver type name
func extractReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	switch t := field.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// ResolveWithAssociatedCode resolves deps AND includes functions/methods for the type
func (g DependencyGraph) ResolveWithAssociatedCode(targetName string) astpkg.Dependencies {
	// Get transitive dependencies
	deps := g.ResolveTransitive(targetName)

	// Find functions that return this type
	functions := g.FindFunctionsReturning(targetName)
	for _, fn := range functions {
		deps.Functions = deps.Functions.Insert(fn)
	}

	// Find methods on this type
	methods := g.FindMethodsForType(targetName)
	for _, method := range methods {
		deps.Functions = deps.Functions.Insert(method)
	}

	// Also resolve dependencies of these functions/methods
	depMonoid := astpkg.NewDependencyMonoid()

	for _, fn := range append(functions, methods...) {
		fnDeps := g.ResolveTransitive(fn)
		deps = depMonoid.Combine(deps, fnDeps)
	}

	return deps
}
