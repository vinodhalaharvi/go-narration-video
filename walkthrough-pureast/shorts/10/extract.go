package extract

import (
	"go/ast"
	"go/token"

	astpkg "github.com/Pure-Company/pureast/pkg/ast"
	"github.com/Pure-Company/purekernels/pkg/fold"
	"github.com/Pure-Company/purekernels/pkg/monoid"
)

// Visitor is our core abstraction: Dependencies -> Dependencies
// This is an endomorphism in the category of Dependencies
type Visitor = func(astpkg.Dependencies) astpkg.Dependencies

// The monoid for composing visitors (function composition)
var visitorMonoid = monoid.NewEndoMonoid[astpkg.Dependencies]()

// Identity visitor (monoid identity)
func identity() Visitor {
	return visitorMonoid.Empty()
}

// Combine visitors using monoid composition
func combineVisitors(v1, v2 Visitor) Visitor {
	return visitorMonoid.Combine(v1, v2)
}

// Pure lifts a Dependencies value into a visitor (always returns that value)
func pure(deps astpkg.Dependencies) Visitor {
	return func(astpkg.Dependencies) astpkg.Dependencies {
		return deps
	}
}

// AddTypeDep creates a visitor that adds a type dependency
func addTypeDep(name string) Visitor {
	return func(d astpkg.Dependencies) astpkg.Dependencies {
		return d.AddType(name)
	}
}

// AddFuncDep creates a visitor that adds a function dependency
func addFuncDep(name string) Visitor {
	return func(d astpkg.Dependencies) astpkg.Dependencies {
		return d.AddFunction(name)
	}
}

// AddImportDep creates a visitor that adds an import dependency
func addImportDep(path string) Visitor {
	return func(d astpkg.Dependencies) astpkg.Dependencies {
		return d.AddImport(path)
	}
}

// FoldVisitors combines multiple visitors using monoid operation
func foldVisitors(visitors []Visitor) Visitor {
	if len(visitors) == 0 {
		return identity()
	}
	return monoid.Reduce(visitorMonoid, visitors)
}

// ExtractType extracts dependencies from a type expression
// Returns a visitor (endomorphism)
func ExtractType(expr ast.Expr) Visitor {
	if expr == nil {
		return identity()
	}

	switch t := expr.(type) {
	case *ast.Ident:
		// Simple type reference
		if isBuiltin(t.Name) {
			return identity()
		}
		return addTypeDep(t.Name)

	case *ast.StarExpr:
		// Pointer type: *T (compose with inner type extraction)
		return ExtractType(t.X)

	case *ast.ArrayType:
		// Array/slice type: []T or [n]T
		return ExtractType(t.Elt)

	case *ast.MapType:
		// Map type: map[K]V (combine both extractions)
		return combineVisitors(
			ExtractType(t.Key),
			ExtractType(t.Value),
		)

	case *ast.ChanType:
		// Channel type: chan T
		return ExtractType(t.Value)

	case *ast.FuncType:
		// Function type: func(args) returns
		return ExtractFuncType(t)

	case *ast.StructType:
		// Inline struct type
		return ExtractStructFields(t.Fields)

	case *ast.InterfaceType:
		// Inline interface type
		return ExtractInterfaceMethods(t.Methods)

	case *ast.SelectorExpr:
		// Qualified type: pkg.Type
		if ident, ok := t.X.(*ast.Ident); ok {
			qualifiedName := ident.Name + "." + t.Sel.Name
			return addTypeDep(qualifiedName)
		}
		return identity()

	case *ast.Ellipsis:
		// Variadic type: ...T
		return ExtractType(t.Elt)

	default:
		return identity()
	}
}

// ExtractFuncType extracts dependencies from function signature
func ExtractFuncType(fn *ast.FuncType) Visitor {
	visitors := []Visitor{}

	// Extract parameter types
	if fn.Params != nil {
		visitors = append(visitors, ExtractFieldList(fn.Params))
	}

	// Extract return types
	if fn.Results != nil {
		visitors = append(visitors, ExtractFieldList(fn.Results))
	}

	// Combine all visitors using monoid
	return foldVisitors(visitors)
}

// ExtractFieldList extracts dependencies from field list
// Uses fold to accumulate visitors, then combines them
func ExtractFieldList(fields *ast.FieldList) Visitor {
	if fields == nil {
		return identity()
	}

	// Map each field to a visitor, then fold them using monoid
	visitors := monoid.Map(
		visitorMonoid,
		func(field *ast.Field) Visitor {
			return ExtractType(field.Type)
		},
		fields.List,
	)

	return func(d astpkg.Dependencies) astpkg.Dependencies {
		return visitors(d)
	}
}

// ExtractStructFields extracts dependencies from struct fields
func ExtractStructFields(fields *ast.FieldList) Visitor {
	if fields == nil {
		return identity()
	}

	// Fold over fields, accumulating visitors
	visitors := fold.Map(
		func(field *ast.Field) Visitor {
			// Extract type dependencies
			typeVisitor := ExtractType(field.Type)

			// Handle embedded fields
			if len(field.Names) == 0 {
				// Embedded field
				if ident, ok := field.Type.(*ast.Ident); ok {
					return combineVisitors(typeVisitor, addTypeDep(ident.Name))
				}
			}

			return typeVisitor
		},
		fields.List,
	)

	return foldVisitors(visitors)
}

// ExtractInterfaceMethods extracts dependencies from interface methods
func ExtractInterfaceMethods(methods *ast.FieldList) Visitor {
	if methods == nil {
		return identity()
	}

	visitors := fold.Map(
		func(method *ast.Field) Visitor {
			// Extract method signature
			if funcType, ok := method.Type.(*ast.FuncType); ok {
				return ExtractFuncType(funcType)
			}

			// Embedded interface
			return ExtractType(method.Type)
		},
		methods.List,
	)

	return foldVisitors(visitors)
}

// ExtractExpr extracts dependencies from an expression
func ExtractExpr(expr ast.Expr) Visitor {
	if expr == nil {
		return identity()
	}

	switch e := expr.(type) {
	case *ast.Ident:
		// Could be a function call or variable reference
		if !isBuiltin(e.Name) {
			return addFuncDep(e.Name)
		}
		return identity()

	case *ast.CallExpr:
		// Function call - combine function and argument visitors
		argVisitors := fold.Map(ExtractExpr, e.Args)
		return combineVisitors(
			ExtractExpr(e.Fun),
			foldVisitors(argVisitors),
		)

	case *ast.SelectorExpr:
		// pkg.Func or obj.Method
		baseVisitor := ExtractExpr(e.X)
		if ident, ok := e.X.(*ast.Ident); ok {
			qualifiedName := ident.Name + "." + e.Sel.Name
			return combineVisitors(baseVisitor, addFuncDep(qualifiedName))
		}
		return baseVisitor

	case *ast.CompositeLit:
		// Composite literal: Type{...}
		typeVisitor := ExtractType(e.Type)
		eltVisitors := fold.Map(ExtractExpr, e.Elts)
		return combineVisitors(typeVisitor, foldVisitors(eltVisitors))

	case *ast.UnaryExpr:
		return ExtractExpr(e.X)

	case *ast.BinaryExpr:
		return combineVisitors(
			ExtractExpr(e.X),
			ExtractExpr(e.Y),
		)

	case *ast.IndexExpr:
		return combineVisitors(
			ExtractExpr(e.X),
			ExtractExpr(e.Index),
		)

	case *ast.SliceExpr:
		visitors := []Visitor{ExtractExpr(e.X)}
		if e.Low != nil {
			visitors = append(visitors, ExtractExpr(e.Low))
		}
		if e.High != nil {
			visitors = append(visitors, ExtractExpr(e.High))
		}
		if e.Max != nil {
			visitors = append(visitors, ExtractExpr(e.Max))
		}
		return foldVisitors(visitors)

	case *ast.TypeAssertExpr:
		baseVisitor := ExtractExpr(e.X)
		if e.Type != nil {
			return combineVisitors(baseVisitor, ExtractType(e.Type))
		}
		return baseVisitor

	case *ast.StarExpr:
		return ExtractExpr(e.X)

	case *ast.ParenExpr:
		return ExtractExpr(e.X)

	case *ast.FuncLit:
		// Function literal
		typeVisitor := ExtractFuncType(e.Type)
		if e.Body != nil {
			return combineVisitors(typeVisitor, ExtractBlockStmt(e.Body))
		}
		return typeVisitor

	default:
		return identity()
	}
}

// ExtractStmt extracts dependencies from a statement
func ExtractStmt(stmt ast.Stmt) Visitor {
	if stmt == nil {
		return identity()
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return ExtractExpr(s.X)

	case *ast.AssignStmt:
		lhsVisitors := fold.Map(ExtractExpr, s.Lhs)
		rhsVisitors := fold.Map(ExtractExpr, s.Rhs)
		return combineVisitors(
			foldVisitors(lhsVisitors),
			foldVisitors(rhsVisitors),
		)

	case *ast.DeclStmt:
		return ExtractDecl(s.Decl)

	case *ast.ReturnStmt:
		visitors := fold.Map(ExtractExpr, s.Results)
		return foldVisitors(visitors)

	case *ast.IfStmt:
		visitors := []Visitor{}
		if s.Init != nil {
			visitors = append(visitors, ExtractStmt(s.Init))
		}
		visitors = append(visitors, ExtractExpr(s.Cond))
		visitors = append(visitors, ExtractBlockStmt(s.Body))
		if s.Else != nil {
			visitors = append(visitors, ExtractStmt(s.Else))
		}
		return foldVisitors(visitors)

	case *ast.ForStmt:
		visitors := []Visitor{}
		if s.Init != nil {
			visitors = append(visitors, ExtractStmt(s.Init))
		}
		if s.Cond != nil {
			visitors = append(visitors, ExtractExpr(s.Cond))
		}
		if s.Post != nil {
			visitors = append(visitors, ExtractStmt(s.Post))
		}
		visitors = append(visitors, ExtractBlockStmt(s.Body))
		return foldVisitors(visitors)

	case *ast.RangeStmt:
		visitors := []Visitor{ExtractExpr(s.X)}
		if s.Key != nil {
			visitors = append(visitors, ExtractExpr(s.Key))
		}
		if s.Value != nil {
			visitors = append(visitors, ExtractExpr(s.Value))
		}
		visitors = append(visitors, ExtractBlockStmt(s.Body))
		return foldVisitors(visitors)

	case *ast.SwitchStmt:
		visitors := []Visitor{}
		if s.Init != nil {
			visitors = append(visitors, ExtractStmt(s.Init))
		}
		if s.Tag != nil {
			visitors = append(visitors, ExtractExpr(s.Tag))
		}
		visitors = append(visitors, ExtractBlockStmt(s.Body))
		return foldVisitors(visitors)

	case *ast.TypeSwitchStmt:
		visitors := []Visitor{}
		if s.Init != nil {
			visitors = append(visitors, ExtractStmt(s.Init))
		}
		visitors = append(visitors, ExtractStmt(s.Assign))
		visitors = append(visitors, ExtractBlockStmt(s.Body))
		return foldVisitors(visitors)

	case *ast.CaseClause:
		exprVisitors := fold.Map(ExtractExpr, s.List)
		stmtVisitors := fold.Map(ExtractStmt, s.Body)
		return combineVisitors(
			foldVisitors(exprVisitors),
			foldVisitors(stmtVisitors),
		)

	case *ast.BlockStmt:
		return ExtractBlockStmt(s)

	case *ast.GoStmt:
		return ExtractExpr(s.Call)

	case *ast.DeferStmt:
		return ExtractExpr(s.Call)

	case *ast.SendStmt:
		return combineVisitors(
			ExtractExpr(s.Chan),
			ExtractExpr(s.Value),
		)

	case *ast.SelectStmt:
		return ExtractBlockStmt(s.Body)

	case *ast.CommClause:
		visitors := []Visitor{}
		if s.Comm != nil {
			visitors = append(visitors, ExtractStmt(s.Comm))
		}
		stmtVisitors := fold.Map(ExtractStmt, s.Body)
		visitors = append(visitors, foldVisitors(stmtVisitors))
		return foldVisitors(visitors)

	default:
		return identity()
	}
}

// ExtractBlockStmt extracts dependencies from a block statement
func ExtractBlockStmt(block *ast.BlockStmt) Visitor {
	if block == nil {
		return identity()
	}

	// Map statements to visitors, then fold using monoid
	visitors := fold.Map(ExtractStmt, block.List)
	return foldVisitors(visitors)
}

// ExtractDecl extracts dependencies from a declaration
func ExtractDecl(decl ast.Decl) Visitor {
	switch d := decl.(type) {
	case *ast.GenDecl:
		return ExtractGenDecl(d)

	case *ast.FuncDecl:
		return ExtractFuncDecl(d)

	default:
		return identity()
	}
}

// ExtractGenDecl extracts dependencies from general declaration
func ExtractGenDecl(decl *ast.GenDecl) Visitor {
	// Handle imports specially
	if decl.Tok == token.IMPORT {
		importVisitors := fold.Map(
			func(spec ast.Spec) Visitor {
				if importSpec, ok := spec.(*ast.ImportSpec); ok {
					path := importSpec.Path.Value
					// Remove quotes
					path = path[1 : len(path)-1]
					return addImportDep(path)
				}
				return identity()
			},
			decl.Specs,
		)
		return foldVisitors(importVisitors)
	}

	// Fold over specs using categorical map
	specVisitors := fold.Map(
		func(spec ast.Spec) Visitor {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				// Extract type definition
				return ExtractType(s.Type)

			case *ast.ValueSpec:
				// Extract variable/constant type and values
				visitors := []Visitor{}
				if s.Type != nil {
					visitors = append(visitors, ExtractType(s.Type))
				}
				valueVisitors := fold.Map(ExtractExpr, s.Values)
				visitors = append(visitors, foldVisitors(valueVisitors))
				return foldVisitors(visitors)

			default:
				return identity()
			}
		},
		decl.Specs,
	)

	return foldVisitors(specVisitors)
}

// ExtractFuncDecl extracts dependencies from function declaration
func ExtractFuncDecl(decl *ast.FuncDecl) Visitor {
	visitors := []Visitor{}

	// Extract receiver type if method
	if decl.Recv != nil {
		visitors = append(visitors, ExtractFieldList(decl.Recv))
	}

	// Extract function signature
	visitors = append(visitors, ExtractFuncType(decl.Type))

	// Extract function body
	if decl.Body != nil {
		visitors = append(visitors, ExtractBlockStmt(decl.Body))
	}

	// Combine all using monoid
	return foldVisitors(visitors)
}

// isBuiltin checks if identifier is a built-in type or function
func isBuiltin(name string) bool {
	builtins := map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true,
		"int8": true, "int16": true, "int32": true, "int64": true,
		"rune": true, "string": true, "uint": true, "uint8": true,
		"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
		"true": true, "false": true, "nil": true, "iota": true,
		"append": true, "cap": true, "close": true, "copy": true,
		"delete": true, "len": true, "make": true, "new": true,
		"panic": true, "print": true, "println": true, "recover": true,
	}
	return builtins[name]
}
