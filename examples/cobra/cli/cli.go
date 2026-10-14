// Package cli is the command tree from skills/cobra/references/commands.md.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// List is the shared core operation that both a CLI and a TUI call.
type List func(context.Context, string) ([]string, error)

// Tags returns known tags for completion. It must be read-only and fast.
type Tags func(context.Context) ([]string, error)

func NewRoot(list List, tags Tags) *cobra.Command {
	root := &cobra.Command{
		Use:           "catalog",
		Short:         "Browse projects",
		SilenceErrors: true, // the process boundary owns error reporting
		SilenceUsage:  true, // explicit --help still works
	}
	var asJSON bool
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "Emit JSON")
	var tag string
	ls := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := list(cmd.Context(), tag)
			if err != nil {
				return err
			}
			if names == nil {
				names = []string{}
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(names)
			}
			for _, name := range names {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	ls.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	// The returned error only reports a missing flag; check it anyway.
	if err := ls.RegisterFlagCompletionFunc("tag", func(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		known, err := tags(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var matches []string
		for _, t := range known {
			if strings.HasPrefix(t, prefix) {
				matches = append(matches, t)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err) // programming error: the flag is defined above
	}
	root.AddCommand(ls)
	return root
}
