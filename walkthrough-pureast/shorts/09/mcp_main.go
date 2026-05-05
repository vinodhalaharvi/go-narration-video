package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/Pure-Company/pureast/pkg/mcp"
)

func main() {
	// Determine worker count
	workers := runtime.NumCPU()
	if workerEnv := os.Getenv("PUREAST_WORKERS"); workerEnv != "" {
		fmt.Sscanf(workerEnv, "%d", &workers)
	}

	// Create MCP server
	server := mcp.NewServer(workers)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "Shutting down MCP server...\n")
		cancel()
	}()

	// Start server in stdio mode (MCP standard)
	fmt.Fprintf(os.Stderr, "PureAST MCP Server starting (workers: %d)...\n", workers)

	if err := server.Serve(ctx, os.Stdin, os.Stdout); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "MCP server stopped\n")
}
