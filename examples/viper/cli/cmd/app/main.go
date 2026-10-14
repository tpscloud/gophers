// Command app shows where the search directories come from. Signals, exit
// codes, and error reporting follow the cobra skill's process boundary.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tpscloud/gophers/examples/viper/cli"
	"github.com/tpscloud/gophers/examples/viper/config"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := cli.NewRoot(searchDirs(), func(_ context.Context, cfg config.Config) error {
		_, err := fmt.Fprintf(stdout, "listening on %s:%d\n", cfg.Server.Addr, cfg.Server.Port)
		return err
	})
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(stderr, "app:", err)
		return 1
	}
	return 0
}

// searchDirs lists the per-user directory first, then the working directory.
// Which directories to search is the application's decision, not Viper's.
func searchDirs() []string {
	var dirs []string
	if dir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(dir, "app"))
	}
	return append(dirs, ".")
}
