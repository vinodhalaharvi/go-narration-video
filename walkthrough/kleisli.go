// Package kleisli provides Kleisli arrows: functions A -> Agent[B].
//
// Why this exists: when chaining LLM calls, every step is a function from
// some input to an effectful computation. The category-theoretic name for
// this is a Kleisli arrow. Composing them gives you pipelines.
//
//   k1: Question -> Agent[Search]
//   k2: Search   -> Agent[Summary]
//   k3: Summary  -> Agent[Email]
//
//   pipeline := Compose(Compose(k1, k2), k3)
//   // pipeline: Question -> Agent[Email]
package kleisli

import (
	"context"

	"agentkit/agent"
)

// Arrow is a function from A to an Agent that produces B.
// Equivalent to Haskell's `a -> m b` in the m=Agent monad.
type Arrow[A, B any] func(A) agent.Agent[B]

// Identity is the identity Kleisli arrow: A -> Agent[A] that just returns its input.
func Identity[A any]() Arrow[A, A] {
	return func(a A) agent.Agent[A] {
		return agent.Pure(a)
	}
}

// Compose composes two Kleisli arrows: (A -> Agent[B]) and (B -> Agent[C])
// produce (A -> Agent[C]).
//
// This is the heart of the design — chaining LLM calls becomes function
// composition, not callback nesting.
func Compose[A, B, C any](f Arrow[A, B], g Arrow[B, C]) Arrow[A, C] {
	return func(a A) agent.Agent[C] {
		return agent.FlatMap(f(a), g)
	}
}

// Pipeline composes a sequence of arrows of the same type into one.
// Useful when you have many steps that all transform the same value type.
func Pipeline[A any](arrows ...Arrow[A, A]) Arrow[A, A] {
	return func(a A) agent.Agent[A] {
		result := agent.Pure(a)
		for _, arrow := range arrows {
			arrow := arrow // capture
			result = agent.FlatMap(result, arrow)
		}
		return result
	}
}

// Lift turns a regular function (A -> B) into a Kleisli arrow that never fails.
// Useful for inserting pure transformations into a pipeline.
func Lift[A, B any](f func(A) B) Arrow[A, B] {
	return func(a A) agent.Agent[B] {
		return agent.Pure(f(a))
	}
}

// LiftErr turns a function that may fail (A -> (B, error)) into a Kleisli arrow.
func LiftErr[A, B any](f func(A) (B, error)) Arrow[A, B] {
	return func(a A) agent.Agent[B] {
		return agent.Agent[B]{
			Name: "liftErr",
			Run: func(ctx context.Context) (B, error) {
				return f(a)
			},
		}
	}
}
