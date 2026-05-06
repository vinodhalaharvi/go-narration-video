package tree

import "fmt"

// Node is a recursive tree: a name, a number, and any children.
type Node struct {
	Name     string
	Number   uint32
	Children []Node
}

// Fold accumulates a value over the tree.
// One generic function. Replaces every "walk this tree and..." you ever wrote.
func Fold[A any](n Node, init A, f func(A, Node) A) A {
	acc := f(init, n)
	for _, child := range n.Children {
		acc = Fold(child, acc, f)
	}
	return acc
}

// Pred is a node predicate. Functions, not interfaces.
type Pred func(Node) bool

// ============================================================
// PRIMITIVES — one Pred per concept, one line each.
// ============================================================

func ByName(name string) Pred {
	return func(n Node) bool { return n.Name == name }
}

func ByNumberAtLeast(min uint32) Pred {
	return func(n Node) bool { return n.Number >= min }
}

func IsLeaf(n Node) bool { return len(n.Children) == 0 }

// ============================================================
// COMBINATORS — Preds in, Pred out. This is composition.
// ============================================================

func And(a, b Pred) Pred { return func(n Node) bool { return a(n) && b(n) } }
func Or(a, b Pred) Pred  { return func(n Node) bool { return a(n) || b(n) } }
func Not(p Pred) Pred    { return func(n Node) bool { return !p(n) } }

// Find returns every node matching pred. Composition of Fold and Pred.
func Find(root Node, pred Pred) []Node {
	return Fold(root, []Node{}, func(acc []Node, n Node) []Node {
		if pred(n) {
			acc = append(acc, n)
		}
		return acc
	})
}

// ============================================================
// DEMO — see it work on a real tree.
// ============================================================

// SampleTree returns a small tree we'll query in different ways.
//
//   root (config, 0)
//   ├── alpha (alpha, 50)
//   │   └── leaf  (alpha, 200)   ← leaf, big number
//   ├── beta  (beta, 30)
//   │   └── leaf  (beta, 80)     ← leaf, small number
//   └── gamma (gamma, 150)        ← leaf, big number
func SampleTree() Node {
	return Node{Name: "config", Number: 0, Children: []Node{
		{Name: "alpha", Number: 50, Children: []Node{
			{Name: "alpha", Number: 200},
		}},
		{Name: "beta", Number: 30, Children: []Node{
			{Name: "beta", Number: 80},
		}},
		{Name: "gamma", Number: 150},
	}}
}

func Demo() {
	root := SampleTree()

	// 1. Find by name. ByName is a primitive Pred.
	alphas := Find(root, ByName("alpha"))
	fmt.Println(len(alphas))
	// → 2  (alpha branch and its leaf)

	// 2. Compose with And. Big leaves only.
	bigLeaves := Find(root, And(IsLeaf, ByNumberAtLeast(100)))
	fmt.Println(len(bigLeaves))
	// → 2  (alpha's leaf with 200, gamma with 150)

	// 3. Compose with Or. Either name.
	pairs := Find(root, Or(ByName("alpha"), ByName("beta")))
	fmt.Println(len(pairs))
	// → 4  (two alphas, two betas)

	// 4. Compose with Not. Branches only.
	branches := Find(root, Not(IsLeaf))
	fmt.Println(len(branches))
	// → 3  (config, alpha, beta)

	// 5. Drop Find. Use Fold directly. Sum all numbers.
	total := Fold(root, uint32(0), func(s uint32, n Node) uint32 {
		return s + n.Number
	})
	fmt.Println(total)
	// → 510  (0+50+200+30+80+150)
}
