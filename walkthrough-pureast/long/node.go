package ast

import (
	"go/ast"

	"github.com/Pure-Company/purekernels/pkg/monoid"
)

// Dependencies accumulates AST dependencies using monoids
// This is our core composition structure - everything combines via monoid operations
type Dependencies struct {
	Types      monoid.SetMonoid[string]
	Functions  monoid.SetMonoid[string]
	Structs    monoid.SetMonoid[string]
	Interfaces monoid.SetMonoid[string]
	Imports    monoid.SetMonoid[string]
	Constants  monoid.SetMonoid[string]
	Variables  monoid.SetMonoid[string]
}

// NewDependencies creates empty dependencies (monoid identity)
func NewDependencies() Dependencies {
	return Dependencies{
		Types:      monoid.NewSetMonoid[string](),
		Functions:  monoid.NewSetMonoid[string](),
		Structs:    monoid.NewSetMonoid[string](),
		Interfaces: monoid.NewSetMonoid[string](),
		Imports:    monoid.NewSetMonoid[string](),
		Constants:  monoid.NewSetMonoid[string](),
		Variables:  monoid.NewSetMonoid[string](),
	}
}

// DependencyMonoid - monoid instance for Dependencies
type DependencyMonoid struct{}

func NewDependencyMonoid() DependencyMonoid {
	return DependencyMonoid{}
}

// Empty returns identity element
func (DependencyMonoid) Empty() Dependencies {
	return NewDependencies()
}

// Combine merges two dependency sets (monoid operation)
func (DependencyMonoid) Combine(a, b Dependencies) Dependencies {
	setMonoid := monoid.NewSetMonoid[string]()

	return Dependencies{
		Types:      setMonoid.Combine(a.Types, b.Types),
		Functions:  setMonoid.Combine(a.Functions, b.Functions),
		Structs:    setMonoid.Combine(a.Structs, b.Structs),
		Interfaces: setMonoid.Combine(a.Interfaces, b.Interfaces),
		Imports:    setMonoid.Combine(a.Imports, b.Imports),
		Constants:  setMonoid.Combine(a.Constants, b.Constants),
		Variables:  setMonoid.Combine(a.Variables, b.Variables),
	}
}

// Helper methods for Dependencies

// AddType adds a type dependency
func (d Dependencies) AddType(name string) Dependencies {
	return Dependencies{
		Types:      d.Types.Insert(name),
		Functions:  d.Functions,
		Structs:    d.Structs,
		Interfaces: d.Interfaces,
		Imports:    d.Imports,
		Constants:  d.Constants,
		Variables:  d.Variables,
	}
}

// AddFunction adds a function dependency
func (d Dependencies) AddFunction(name string) Dependencies {
	return Dependencies{
		Types:      d.Types,
		Functions:  d.Functions.Insert(name),
		Structs:    d.Structs,
		Interfaces: d.Interfaces,
		Imports:    d.Imports,
		Constants:  d.Constants,
		Variables:  d.Variables,
	}
}

// AddStruct adds a struct dependency
func (d Dependencies) AddStruct(name string) Dependencies {
	return Dependencies{
		Types:      d.Types,
		Functions:  d.Functions,
		Structs:    d.Structs.Insert(name),
		Interfaces: d.Interfaces,
		Imports:    d.Imports,
		Constants:  d.Constants,
		Variables:  d.Variables,
	}
}

// AddInterface adds an interface dependency
func (d Dependencies) AddInterface(name string) Dependencies {
	return Dependencies{
		Types:      d.Types,
		Functions:  d.Functions,
		Structs:    d.Structs,
		Interfaces: d.Interfaces.Insert(name),
		Imports:    d.Imports,
		Constants:  d.Constants,
		Variables:  d.Variables,
	}
}

// AddImport adds an import dependency
func (d Dependencies) AddImport(path string) Dependencies {
	return Dependencies{
		Types:      d.Types,
		Functions:  d.Functions,
		Structs:    d.Structs,
		Interfaces: d.Interfaces,
		Imports:    d.Imports.Insert(path),
		Constants:  d.Constants,
		Variables:  d.Variables,
	}
}

// Combine merges with another Dependencies using monoid operation
func (d Dependencies) Combine(other Dependencies) Dependencies {
	m := NewDependencyMonoid()
	return m.Combine(d, other)
}

// IsEmpty checks if dependencies are empty
func (d Dependencies) IsEmpty() bool {
	return d.Types.Size() == 0 &&
		d.Functions.Size() == 0 &&
		d.Structs.Size() == 0 &&
		d.Interfaces.Size() == 0 &&
		d.Imports.Size() == 0 &&
		d.Constants.Size() == 0 &&
		d.Variables.Size() == 0
}

// DeclNode represents a declaration with its dependencies
type DeclNode struct {
	Name string
	Decl ast.Decl
	Deps Dependencies
}

// FileNode represents a file with all its declarations
type FileNode struct {
	Name    string
	File    *ast.File
	Decls   []DeclNode
	Imports []string
	Deps    Dependencies
}

// PackageNode represents a package with multiple files
type PackageNode struct {
	Name  string
	Files []FileNode
	Deps  Dependencies
}

// MethodNode represents a method with receiver
type MethodNode struct {
	ReceiverType string
	MethodName   string
	Func         *ast.FuncDecl
	Deps         Dependencies
}

// InterfaceImplementation tracks interface implementation
type InterfaceImplementation struct {
	TypeName       string
	InterfaceName  string
	Methods        []MethodNode
	MissingMethods []string
}
