// Package main demonstrates a multi-step research agent built from
// composable Agent primitives.
//
// The pipeline:
//   Question
//     → Search   (LLM generates search queries)
//     → Findings (LLM extracts facts from results)
//     → Summary  (LLM synthesizes a summary)
//
// Each step is an Agent. We compose them with Kleisli arrows so the whole
// pipeline reads like function composition, not callback nesting.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"agentkit/agent"
	"agentkit/kleisli"
)

// ============================================================
// Domain types
// ============================================================

type Question string

type SearchQueries struct {
	Original Question
	Queries  []string
}

type Findings struct {
	Question Question
	Facts    []string
}

type Summary struct {
	Question Question
	Text     string
	Sources  int
}

// ============================================================
// Mock LLM client — would be a real client in production
// ============================================================

type LLM interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type fakeLLM struct {
	responses map[string]string
	delay     time.Duration
}

func (f *fakeLLM) Complete(ctx context.Context, prompt string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(f.delay):
	}
	for keyword, resp := range f.responses {
		if strings.Contains(prompt, keyword) {
			return resp, nil
		}
	}
	return "(no response)", nil
}

// ============================================================
// Step 1: Question → SearchQueries
// ============================================================

func generateQueries(llm LLM) kleisli.Arrow[Question, SearchQueries] {
	return func(q Question) agent.Agent[SearchQueries] {
		return agent.Agent[SearchQueries]{
			Name: "generateQueries",
			Run: func(ctx context.Context) (SearchQueries, error) {
				prompt := fmt.Sprintf("Generate 3 search queries for: %s", q)
				resp, err := llm.Complete(ctx, prompt)
				if err != nil {
					return SearchQueries{}, fmt.Errorf("generateQueries: %w", err)
				}
				queries := strings.Split(resp, "\n")
				return SearchQueries{Original: q, Queries: queries}, nil
			},
		}
	}
}

// ============================================================
// Step 2: SearchQueries → Findings
//
// This step runs the queries in parallel using Parallel2.
// In real life you'd use a fan-out helper for N queries; we use 2 here
// to keep the example readable.
// ============================================================

func searchOne(llm LLM, query string) agent.Agent[[]string] {
	return agent.Agent[[]string]{
		Name: "search:" + query,
		Run: func(ctx context.Context) ([]string, error) {
			resp, err := llm.Complete(ctx, "Search results for: "+query)
			if err != nil {
				return nil, err
			}
			return strings.Split(resp, "\n"), nil
		},
	}
}

func gatherFindings(llm LLM) kleisli.Arrow[SearchQueries, Findings] {
	return func(sq SearchQueries) agent.Agent[Findings] {
		// Take first two queries and run them in parallel
		q1 := sq.Queries[0]
		q2 := q1
		if len(sq.Queries) > 1 {
			q2 = sq.Queries[1]
		}
		return agent.Map(
			agent.Parallel2(
				searchOne(llm, q1),
				searchOne(llm, q2),
				func(a, b []string) []string {
					return append(a, b...)
				},
			),
			func(facts []string) Findings {
				return Findings{Question: sq.Original, Facts: facts}
			},
		)
	}
}

// ============================================================
// Step 3: Findings → Summary
// ============================================================

func synthesize(llm LLM) kleisli.Arrow[Findings, Summary] {
	return func(f Findings) agent.Agent[Summary] {
		return agent.Agent[Summary]{
			Name: "synthesize",
			Run: func(ctx context.Context) (Summary, error) {
				prompt := fmt.Sprintf(
					"Synthesize a summary for %q from %d facts",
					f.Question, len(f.Facts),
				)
				resp, err := llm.Complete(ctx, prompt)
				if err != nil {
					return Summary{}, err
				}
				return Summary{
					Question: f.Question,
					Text:     resp,
					Sources:  len(f.Facts),
				}, nil
			},
		}
	}
}

// ============================================================
// The whole pipeline, composed as Kleisli arrows.
//
// This is the payoff: the entire agent is one expression. No
// callback hell, no plumbing, no glue code.
// ============================================================

func researchPipeline(llm LLM) kleisli.Arrow[Question, Summary] {
	return kleisli.Compose(
		kleisli.Compose(
			generateQueries(llm),
			gatherFindings(llm),
		),
		synthesize(llm),
	)
}

// ============================================================
// Run it
// ============================================================

func main() {
	llm := &fakeLLM{
		delay: 100 * time.Millisecond,
		responses: map[string]string{
			"Generate 3 search queries": "queries about Go generics\nGo type parameters\nGo 1.18 features",
			"Search results":            "Go added generics in 1.18\nType parameters work like in Java\nConstraints replace interfaces",
			"Synthesize":                "Go added generics in version 1.18 via type parameters, constrained by interfaces.",
		},
	}

	pipeline := researchPipeline(llm)

	// Apply cross-cutting concerns by wrapping the final agent.
	// Note how retry/timeout/trace compose without touching the pipeline logic.
	question := Question("How do Go generics work?")
	job := agent.Trace(
		agent.WithTimeout(
			agent.WithRetry(
				pipeline(question),
				3,
				50*time.Millisecond,
			),
			5*time.Second,
		),
		log.Printf,
	)

	ctx := context.Background()
	summary, err := job.Run(ctx)
	if err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}

	fmt.Println()
	fmt.Println("=== RESULT ===")
	fmt.Printf("Question: %s\n", summary.Question)
	fmt.Printf("Sources:  %d\n", summary.Sources)
	fmt.Printf("Summary:  %s\n", summary.Text)
}
