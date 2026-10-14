// Command catalog shows the process boundary from
// skills/cobra/references/commands.md: signals, one error report, exit codes.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	cli "github.com/tpscloud/gophers/examples/cobra/cli"
)

// version is set at build time:
// go build -ldflags "-X main.version=1.2.3" ./cli/cmd/catalog
var version = "dev"

const (
	exitOK          = 0
	exitFailure     = 1
	exitInterrupted = 130
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run owns deferred cleanup and returns the exit code, so main can call
// os.Exit without skipping defers.
func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRoot(listProjects, knownTags)
	root.Version = version
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return exitInterrupted
		}
		fmt.Fprintln(stderr, "catalog:", err)
		return exitFailure
	}
	return exitOK
}

func listProjects(ctx context.Context, tag string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projects := map[string][]string{"go": {"cobra", "pflag"}, "": {"cobra", "pflag", "ripgrep"}}
	return projects[tag], nil
}

func knownTags(context.Context) ([]string, error) { return []string{"go", "rust"}, nil }
