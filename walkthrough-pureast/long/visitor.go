package ast

import (
	"go/ast"
)

// Visitor is a function that visits an AST node and accumulates dependencies
// This follows the fold pattern: (Dependencies, Node) -> Dependencies
type Visitor func(Dependencies) Dependencies

// Identity visitor - returns dependencies unchanged
func Identity() Visitor {
	return func(d Dependencies) Dependencies {
		return d
	}
}

// ComposeVisitors combines multiple visitors (function composition)
func ComposeVisitors(visitors ...Visitor) Visitor {
	return func(d Dependencies) Dependencies {
		result := d
		for _, v := range visitors {
			result = v(result)
		}
		return result
	}
}

// Visit applies a visitor to extract dependencies
func Visit(node ast.Node, visitor Visitor) Dependencies {
	return visitor(NewDependencies())
}

// ExtractorFunc is a function that extracts dependencies from an AST node
// This is our main abstraction for AST traversal
type ExtractorFunc func(ast.Node) Visitor
