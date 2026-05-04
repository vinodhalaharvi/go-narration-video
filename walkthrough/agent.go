// Package agent provides the core Agent[A] type — an effectful computation
// that produces a value of type A or an error, given a context.
//
// Agent forms a Functor, Applicative, and Monad. Compose with Map, Ap, FlatMap.
package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Agent is an effectful computation producing A.
//
// Think of it as: "a recipe that, when run, will produce an A or fail."
// Two Agents are composable without being run — composition is a pure
// data-structure operation. They only execute when Run is called.
type Agent[A any] struct {
	// Name is used for tracing and debugging composed pipelines.
	Name string

	// Run executes the agent. The context controls cancellation and deadlines.
	Run func(ctx context.Context) (A, error)
}

// Pure lifts a plain value into an Agent that simply returns it.
// This is the unit/return of the monad.
func Pure[A any](a A) Agent[A] {
	return Agent[A]{
		Name: "pure",
		Run: func(ctx context.Context) (A, error) {
			return a, nil
		},
	}
}

// Fail constructs an Agent that always fails with err.
func Fail[A any](err error) Agent[A] {
	return Agent[A]{
		Name: "fail",
		Run: func(ctx context.Context) (A, error) {
			var zero A
			return zero, err
		},
	}
}

// Map applies f to the result of agent. This is Functor.fmap.
//
// Laws (must hold for any well-behaved functor):
//   Map(a, identity) == a
//   Map(Map(a, f), g) == Map(a, compose(g, f))
func Map[A, B any](a Agent[A], f func(A) B) Agent[B] {
	return Agent[B]{
		Name: a.Name + ".map",
		Run: func(ctx context.Context) (B, error) {
			v, err := a.Run(ctx)
			if err != nil {
				var zero B
				return zero, err
			}
			return f(v), nil
		},
	}
}

// FlatMap chains a function that itself returns an Agent. This is Monad.bind.
//
// Use when the next step depends on the result of the previous step
// (e.g., "ask the LLM what to do, then do that thing").
//
// Law: FlatMap is associative —
//   FlatMap(FlatMap(a, f), g) == FlatMap(a, λx. FlatMap(f(x), g))
func FlatMap[A, B any](a Agent[A], f func(A) Agent[B]) Agent[B] {
	return Agent[B]{
		Name: a.Name + ".flatMap",
		Run: func(ctx context.Context) (B, error) {
			v, err := a.Run(ctx)
			if err != nil {
				var zero B
				return zero, err
			}
			return f(v).Run(ctx)
		},
	}
}

// Ap is applicative apply: combine an Agent of a function with an Agent of
// a value. This is the missing piece that lets you compose N independent
// agents in parallel without sequencing.
//
// For most practical use, prefer Map2/Map3 below.
func Ap[A, B any](af Agent[func(A) B], aa Agent[A]) Agent[B] {
	return Agent[B]{
		Name: af.Name + ".ap",
		Run: func(ctx context.Context) (B, error) {
			f, err := af.Run(ctx)
			if err != nil {
				var zero B
				return zero, err
			}
			return Map(aa, f).Run(ctx)
		},
	}
}

// Map2 combines two independent agents with a binary function.
// Both agents run sequentially (use Parallel2 to run them concurrently).
func Map2[A, B, C any](a Agent[A], b Agent[B], f func(A, B) C) Agent[C] {
	return Agent[C]{
		Name: a.Name + "+" + b.Name,
		Run: func(ctx context.Context) (C, error) {
			va, err := a.Run(ctx)
			if err != nil {
				var zero C
				return zero, err
			}
			vb, err := b.Run(ctx)
			if err != nil {
				var zero C
				return zero, err
			}
			return f(va, vb), nil
		},
	}
}

// Parallel2 runs two agents concurrently and combines their results.
// If either fails, the context is canceled to abort the other.
func Parallel2[A, B, C any](a Agent[A], b Agent[B], f func(A, B) C) Agent[C] {
	return Agent[C]{
		Name: a.Name + "||" + b.Name,
		Run: func(ctx context.Context) (C, error) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			var (
				va     A
				vb     B
				errA   error
				errB   error
				wg     sync.WaitGroup
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				va, errA = a.Run(ctx)
				if errA != nil {
					cancel()
				}
			}()
			go func() {
				defer wg.Done()
				vb, errB = b.Run(ctx)
				if errB != nil {
					cancel()
				}
			}()
			wg.Wait()

			if errA != nil {
				var zero C
				return zero, errA
			}
			if errB != nil {
				var zero C
				return zero, errB
			}
			return f(va, vb), nil
		},
	}
}

// WithRetry wraps an agent so it retries up to maxAttempts on failure,
// with exponential backoff starting at baseDelay.
//
// Composition note: this is itself a function Agent[A] -> Agent[A], so it
// composes naturally with Map, FlatMap, etc.
func WithRetry[A any](a Agent[A], maxAttempts int, baseDelay time.Duration) Agent[A] {
	return Agent[A]{
		Name: a.Name + ".retry",
		Run: func(ctx context.Context) (A, error) {
			var lastErr error
			delay := baseDelay
			for attempt := 0; attempt < maxAttempts; attempt++ {
				if attempt > 0 {
					select {
					case <-ctx.Done():
						var zero A
						return zero, ctx.Err()
					case <-time.After(delay):
					}
					delay *= 2
				}
				v, err := a.Run(ctx)
				if err == nil {
					return v, nil
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return v, err
				}
				lastErr = err
			}
			var zero A
			return zero, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
		},
	}
}

// WithTimeout bounds an agent's execution time.
func WithTimeout[A any](a Agent[A], d time.Duration) Agent[A] {
	return Agent[A]{
		Name: a.Name + ".timeout",
		Run: func(ctx context.Context) (A, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return a.Run(ctx)
		},
	}
}

// Trace wraps an agent to log timing and result. Useful for debugging
// pipelines composed of many agents.
func Trace[A any](a Agent[A], log func(format string, args ...any)) Agent[A] {
	return Agent[A]{
		Name: a.Name + ".trace",
		Run: func(ctx context.Context) (A, error) {
			start := time.Now()
			log("→ %s starting", a.Name)
			v, err := a.Run(ctx)
			elapsed := time.Since(start)
			if err != nil {
				log("✗ %s failed in %v: %v", a.Name, elapsed, err)
			} else {
				log("✓ %s done in %v", a.Name, elapsed)
			}
			return v, err
		},
	}
}
