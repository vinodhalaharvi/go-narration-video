package main

import "fmt"

// Functor over a slice: lifts A -> B into []A -> []B
func Map[A, B any](xs []A, f func(A) B) []B {
	out := make([]B, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func main() {
	nums := []int{1, 2, 3, 4, 5}
	squared := Map(nums, func(n int) int {
		return n * n
	})
	fmt.Println(squared)
}
