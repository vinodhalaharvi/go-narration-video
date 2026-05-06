package tree

// Node is a recursive tree: a name, a number, and any children.
type Node struct {
	Name     string
	Number   uint32
	Children []Node
}

// Fold accumulates a value over the tree.
// One function. Replaces every "walk this tree and..." you ever wrote.
func Fold[A any](n Node, init A, f func(A, Node) A) A {
	acc := f(init, n)
	for _, child := range n.Children {
		acc = Fold(child, acc, f)
	}
	return acc
}

// Pred is a node predicate. Functions, not interfaces.
type Pred func(Node) bool

func ByName(name string) Pred {
	return func(n Node) bool { return n.Name == name }
}

// Find returns every node matching pred. Composition of Fold and Pred.
func Find(root Node, pred Pred) []Node {
	return Fold(root, []Node{}, func(acc []Node, n Node) []Node {
		if pred(n) {
			acc = append(acc, n)
		}
		return acc
	})
}
