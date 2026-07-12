// Command nasij is the NASIJ CLI binary entry point.
//
// Build with version injection:
//
//	go build -ldflags "-X github.com/nasij/nasij/pkg/version.Version=0.1.0" ./cmd/nasij
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nasij/nasij/internal/cli"
	"github.com/nasij/nasij/internal/container"
)

func main() {
	ctx := context.Background()

	// Resolve --config flag value before building the container,
	// so the container receives the correct config path.
	cfgPath := resolveConfigFlag(os.Args[1:])

	c, err := container.New(ctx, cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nasij: fatal: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	root := cli.NewRoot(c)

	if err := root.ExecuteContext(ctx); err != nil {
		// Cobra writes the error to stderr; we just set the exit code.
		os.Exit(1)
	}
}

// resolveConfigFlag extracts the value of --config or -c from raw args
// before cobra parses them, so the container can be built with the
// correct config path.
func resolveConfigFlag(args []string) string {
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
