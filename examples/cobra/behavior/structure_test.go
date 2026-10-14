package behavior

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// references/commands.md: groups are resolved at execution, so AddGroup may
// come before or after AddCommand. An unknown GroupID panics on execute.
func TestGroupOrderDoesNotMatter(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	root.AddCommand(&cobra.Command{Use: "serve", GroupID: "core", RunE: noop})
	root.AddGroup(&cobra.Group{ID: "core", Title: "Core commands:"})
	out, _, err := run(root, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Core commands:\n  serve") {
		t.Fatalf("help=%q", out)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unknown group ID")
		}
	}()
	bad := &cobra.Command{Use: "app"}
	bad.AddCommand(&cobra.Command{Use: "serve", GroupID: "missing", RunE: noop})
	run(bad, "--help")
}

// references/commands.md: setting Version adds --version, printed to the out
// writer; SetVersionTemplate changes the format. Build tools set the value
// with -ldflags "-X main.version=...".
func TestVersionFlag(t *testing.T) {
	root := &cobra.Command{Use: "app", Version: "1.2.3", RunE: noop}
	out, _, err := run(root, "--version")
	if err != nil || out != "app version 1.2.3\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	root = &cobra.Command{Use: "app", Version: "1.2.3", RunE: noop}
	root.SetVersionTemplate("{{.Version}}\n")
	if out, _, _ := run(root, "--version"); out != "1.2.3\n" {
		t.Fatalf("templated out=%q", out)
	}
}

// references/commands.md: Hidden removes a command from help but it still
// runs; Deprecated also hides it and prints a notice when it runs.
func TestHiddenAndDeprecated(t *testing.T) {
	var ran []string
	mk := func() *cobra.Command {
		root := &cobra.Command{Use: "app"}
		root.AddCommand(
			&cobra.Command{Use: "visible", RunE: noop},
			&cobra.Command{Use: "secret", Hidden: true, RunE: func(*cobra.Command, []string) error { ran = append(ran, "secret"); return nil }},
			&cobra.Command{Use: "old", Deprecated: "use visible instead", RunE: func(*cobra.Command, []string) error { ran = append(ran, "old"); return nil }},
		)
		return root
	}
	help, _, err := run(mk(), "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(help, "secret") || strings.Contains(help, "old") || !strings.Contains(help, "visible") {
		t.Fatalf("help=%q", help)
	}
	if _, _, err := run(mk(), "secret"); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(mk(), "old")
	if err != nil || !strings.Contains(out, "deprecated, use visible instead") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if strings.Join(ran, ",") != "secret,old" {
		t.Fatalf("ran=%v", ran)
	}
}

// Flags and lifecycle pitfalls: Execute on a subcommand runs the whole tree
// from the root, so tests must set args on the root, not call child.Execute.
func TestSubcommandExecuteRunsFromRoot(t *testing.T) {
	var got string
	root := &cobra.Command{Use: "app", RunE: func(*cobra.Command, []string) error { got = "root"; return nil }}
	child := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { got = "child"; return nil }}
	root.AddCommand(child)
	child.SetArgs([]string{})
	child.SetOut(new(strings.Builder))
	if err := child.Execute(); err != nil {
		t.Fatal(err)
	}
	if got != "root" {
		t.Fatalf("child.Execute ran %q", got)
	}
}

// references/commands.md: MarkFlagsRequiredTogether and MarkFlagsOneRequired,
// like required flags, look only at flags the user set.
func TestFlagGroups(t *testing.T) {
	mk := func() *cobra.Command {
		c := &cobra.Command{Use: "login", RunE: noop, SilenceErrors: true, SilenceUsage: true}
		c.Flags().String("user", "", "")
		c.Flags().String("password", "", "")
		c.Flags().String("token", "default-token", "")
		c.MarkFlagsRequiredTogether("user", "password")
		c.MarkFlagsOneRequired("user", "token")
		return c
	}
	if _, _, err := run(mk(), "--user", "a"); err == nil || !strings.Contains(err.Error(), "must all be set") {
		t.Fatalf("together: %v", err)
	}
	// The token default does not satisfy MarkFlagsOneRequired.
	if _, _, err := run(mk()); err == nil || !strings.Contains(err.Error(), "at least one of the flags") {
		t.Fatalf("one required: %v", err)
	}
	if _, _, err := run(mk(), "--token", "t"); err != nil {
		t.Fatal(err)
	}
}

// references/completion.md: MarkFlagFilename and MarkFlagDirname set the
// shell file-completion directive for a flag.
func TestFilenameCompletion(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	sub := &cobra.Command{Use: "load", RunE: noop}
	sub.Flags().String("file", "", "")
	sub.Flags().String("dir", "", "")
	if err := errors.Join(sub.MarkFlagFilename("file", "yaml", "yml"), sub.MarkFlagDirname("dir")); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(sub)
	out, _, err := run(root, "__complete", "load", "--file", "")
	if err != nil || out != "yaml\nyml\n:8\n" {
		t.Fatalf("file out=%q err=%v", out, err)
	}
	out, _, err = run(root, "__complete", "load", "--dir", "")
	if err != nil || out != ":16\n" {
		t.Fatalf("dir out=%q err=%v", out, err)
	}
}
