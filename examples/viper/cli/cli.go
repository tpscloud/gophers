// Package cli wires the config loader into a Cobra command tree. Only the
// command that needs settings loads them, so help and shell completion work
// on a machine with no config file or a broken one.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/tpscloud/gophers/examples/viper/config"
)

// Serve is the core operation. It receives a validated snapshot, never a
// *viper.Viper.
type Serve func(context.Context, config.Config) error

// NewRoot builds a fresh tree. dirs are the config search directories, in
// order; main passes the user's config directory.
func NewRoot(dirs []string, serve Serve) *cobra.Command {
	root := &cobra.Command{
		Use:           "app",
		Short:         "Serve the app",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	var file string
	root.PersistentFlags().StringVar(&file, "config", "", "config file (default: the first app.yaml in the search directories)")

	srv := &cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		Args:  cobra.NoArgs,
		// Load here, not in a root PersistentPreRunE: Cobra runs the root
		// hook for `completion`, `__complete`, and `help <command>` too.
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(config.Source{File: file, Dirs: dirs, Flags: cmd.Flags()})
			if err != nil {
				return err
			}
			return serve(cmd.Context(), cfg)
		},
	}
	config.AddFlags(srv.Flags())
	if err := srv.RegisterFlagCompletionFunc("log-level", cobra.FixedCompletions(
		[]string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp,
	)); err != nil {
		panic(err) // programming error: AddFlags registers log-level
	}
	root.AddCommand(srv)
	return root
}
