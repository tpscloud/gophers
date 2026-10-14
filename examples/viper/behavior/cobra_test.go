package behavior

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

// SKILL.md: a root PersistentPreRunE that loads config also runs for
// completion requests, so a broken or missing config breaks tab completion.
func TestRootHookRunsForCompletion(t *testing.T) {
	for _, args := range [][]string{
		{"__complete", "serve", ""},
		{"completion", "bash"},
		{"help", "serve"},
	} {
		var ran bool
		root := &cobra.Command{
			Use:           "app",
			SilenceErrors: true,
			SilenceUsage:  true,
			PersistentPreRunE: func(*cobra.Command, []string) error {
				ran = true
				return nil
			},
		}
		root.AddCommand(&cobra.Command{Use: "serve", RunE: func(*cobra.Command, []string) error { return nil }})
		root.SetArgs(args)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !ran {
			t.Errorf("%v: root hook did not run", args)
		}
	}
}
