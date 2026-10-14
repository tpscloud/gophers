// Package behavior checks the Cobra v1.10.2 behavior that skills/cobra
// states as fact. When a test here fails after a Cobra upgrade, update the
// skill text before changing the test.
package behavior

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func run(root *cobra.Command, args ...string) (string, string, error) {
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

func noop(*cobra.Command, []string) error { return nil }

// Flags and lifecycle pitfalls: nearest persistent hook wins, including the command's own.
func TestNearestPersistentHookWins(t *testing.T) {
	var calls []string
	root := &cobra.Command{Use: "app", PersistentPreRunE: func(*cobra.Command, []string) error { calls = append(calls, "root"); return nil }}
	child := &cobra.Command{Use: "child", RunE: noop, PersistentPreRunE: func(*cobra.Command, []string) error { calls = append(calls, "child"); return nil }}
	root.AddCommand(child)
	if _, _, err := run(root, "child"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"child"}) {
		t.Fatalf("calls=%v", calls)
	}
}

// Flags and lifecycle pitfalls: EnableTraverseRunHooks runs pre-hooks root to leaf
// and post-hooks leaf to root.
func TestTraverseRunHooksOrder(t *testing.T) {
	cobra.EnableTraverseRunHooks = true
	defer func() { cobra.EnableTraverseRunHooks = false }()
	var calls []string
	mk := func(n string) func(*cobra.Command, []string) error {
		return func(*cobra.Command, []string) error { calls = append(calls, n); return nil }
	}
	root := &cobra.Command{Use: "app", PersistentPreRunE: mk("pre-root"), PersistentPostRunE: mk("post-root")}
	child := &cobra.Command{Use: "child", RunE: mk("run"), PersistentPreRunE: mk("pre-child"), PersistentPostRunE: mk("post-child")}
	root.AddCommand(child)
	if _, _, err := run(root, "child"); err != nil {
		t.Fatal(err)
	}
	want := []string{"pre-root", "pre-child", "run", "post-child", "post-root"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls=%v", calls)
	}
}

// Flags and lifecycle pitfalls: pre-hooks run before required-flag validation; Args validation runs before both.
func TestPreHooksBeforeRequiredFlags(t *testing.T) {
	var pre, ran bool
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	child := &cobra.Command{Use: "child", Args: cobra.NoArgs,
		PreRunE: func(*cobra.Command, []string) error { pre = true; return nil },
		RunE:    func(*cobra.Command, []string) error { ran = true; return nil }}
	child.Flags().String("name", "default-not-empty", "")
	if err := child.MarkFlagRequired("name"); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(child)
	_, _, err := run(root, "child")
	if err == nil || !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("err=%v", err)
	}
	if !pre || ran {
		t.Fatalf("pre=%v ran=%v", pre, ran)
	}
	pre = false
	if _, _, err := run(root, "child", "extra"); err == nil || pre {
		t.Fatalf("args validation should fail before pre-hook: err=%v pre=%v", err, pre)
	}
}

// Flags and lifecycle pitfalls: post-hooks are skipped after a failed RunE.
func TestPostHooksSkippedOnRunError(t *testing.T) {
	var post, ppost bool
	boom := errors.New("boom")
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true,
		PersistentPostRunE: func(*cobra.Command, []string) error { ppost = true; return nil }}
	child := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return boom },
		PostRunE: func(*cobra.Command, []string) error { post = true; return nil }}
	root.AddCommand(child)
	if _, _, err := run(root, "child"); !errors.Is(err, boom) || post || ppost {
		t.Fatalf("err=%v post=%v ppost=%v", err, post, ppost)
	}
}

// Flags and lifecycle pitfalls: group helpers panic on an undefined flag.
func TestFlagGroupPanicsOnUndefinedFlag(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	c := &cobra.Command{Use: "x"}
	c.Flags().Bool("a", false, "")
	c.MarkFlagsMutuallyExclusive("a", "missing")
}

// Implementation choices: a missing Args validator: leaf accepts anything, a root with
// subcommands rejects an unknown word, a nested group accepts it silently.
func TestMissingArgsValidator(t *testing.T) {
	var got []string
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	leaf := &cobra.Command{Use: "leaf", RunE: func(_ *cobra.Command, a []string) error { got = a; return nil }}
	group := &cobra.Command{Use: "group"}
	group.AddCommand(&cobra.Command{Use: "sub", RunE: noop})
	root.AddCommand(leaf, group)
	if _, _, err := run(root, "leaf", "a", "b"); err != nil || len(got) != 2 {
		t.Fatalf("leaf err=%v got=%v", err, got)
	}
	if _, _, err := run(root, "typo"); err == nil {
		t.Fatal("root should reject unknown command")
	}
	out, errb, err := run(root, "group", "typo")
	t.Logf("nested group typo: err=%v out=%d bytes stderr=%d bytes", err, len(out), len(errb))
	if err != nil {
		t.Fatalf("nested group with a typo returned %v; expected silent help", err)
	}
}

// Implementation choices: Print* follows the *out* writer when SetOut was called and
// otherwise falls back to the process stderr, never to the SetErr writer.
// A test that calls SetOut therefore sees Println output as payload while
// the real binary, which calls neither setter, sends it to stderr.
func TestPrintFollowsOutWriterElseProcessStderr(t *testing.T) {
	var out, errb bytes.Buffer
	c := &cobra.Command{Use: "x", RunE: func(cmd *cobra.Command, _ []string) error { cmd.Println("payload"); return nil }}
	c.SetOut(&out)
	c.SetErr(&errb)
	c.SetArgs([]string{})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "payload\n" || errb.Len() != 0 {
		t.Fatalf("out=%q err=%q", out.String(), errb.String())
	}
	if w := (&cobra.Command{}).OutOrStderr(); w != os.Stderr {
		t.Fatalf("default Print writer is %v, want os.Stderr", w)
	}
}

// references/commands.md: root-level SilenceErrors/SilenceUsage cover descendants.
func TestRootSilenceCoversChildren(t *testing.T) {
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(&cobra.Command{Use: "leaf", Args: cobra.NoArgs, RunE: noop})
	out, errb, err := run(root, "leaf", "extra")
	if err == nil || out != "" || errb != "" {
		t.Fatalf("err=%v out=%q stderr=%q", err, out, errb)
	}
}

// references/completion.md: the default completion command and
// the __complete protocol can be tested through SetArgs.
func TestCompletionProtocol(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: noop}
	list.Flags().String("tag", "", "")
	if err := list.RegisterFlagCompletionFunc("tag", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"go", "git"}, cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		t.Fatal(err)
	}
	root.AddCommand(list)
	out, errb, err := run(root, "__complete", "list", "--tag", "g")
	if err != nil {
		t.Fatal(err)
	}
	if out != "go\ngit\n:4\n" {
		t.Fatalf("stdout=%q", out)
	}
	t.Logf("stderr=%q", errb)
	if c, _, _ := root.Find([]string{"completion"}); c == nil || c.Name() != "completion" {
		t.Fatal("default completion command missing")
	}
	// A program without subcommands still gets the default completion command
	// in v1.10.2, but only when "completion" is the word being invoked. The
	// root's own RunE never sees that positional argument.
	var got []string
	solo := &cobra.Command{Use: "solo", RunE: func(_ *cobra.Command, a []string) error { got = a; return nil }}
	out, _, err = run(solo, "completion")
	if err != nil || got != nil || !strings.Contains(out, "autocompletion script") {
		t.Fatalf("out=%q got=%v err=%v", out, got, err)
	}
	solo = &cobra.Command{Use: "solo", RunE: func(_ *cobra.Command, a []string) error { got = a; return nil }}
	if _, _, err := run(solo, "other"); err != nil || !slices.Equal(got, []string{"other"}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// Implementation choices: parent local flags are not inherited.
// Flags and lifecycle pitfalls: StringSlice splits on commas, StringArray does not.
func TestFlagScopeAndSliceTypes(t *testing.T) {
	var slice, array []string
	root := &cobra.Command{Use: "app", SilenceErrors: true, SilenceUsage: true}
	root.Flags().Bool("local", false, "")
	leaf := &cobra.Command{Use: "leaf", RunE: noop}
	leaf.Flags().StringSliceVar(&slice, "s", nil, "")
	leaf.Flags().StringArrayVar(&array, "a", nil, "")
	root.AddCommand(leaf)
	if _, _, err := run(root, "leaf", "--local"); err == nil {
		t.Fatal("parent local flag should be unknown on the child")
	}
	if _, _, err := run(root, "leaf", "--s", "x,y", "--a", "x,y"); err != nil {
		t.Fatal(err)
	}
	if len(slice) != 2 || len(array) != 1 {
		t.Fatalf("slice=%v array=%v", slice, array)
	}
}

// Flags and lifecycle pitfalls: help, version and completion should not need valid config.
// A root PersistentPreRunE that fails (for example on a missing config file)
// is skipped for --help and --version but does run for the generated
// completion subcommands, every __complete request, and help <command>.
func TestRootPersistentHookRunsForCompletion(t *testing.T) {
	bad := errors.New("config missing")
	mk := func() *cobra.Command {
		root := &cobra.Command{Use: "app", Version: "1", SilenceErrors: true, SilenceUsage: true,
			PersistentPreRunE: func(*cobra.Command, []string) error { return bad }}
		root.AddCommand(&cobra.Command{Use: "list", RunE: noop})
		return root
	}
	for _, args := range [][]string{{"--help"}, {"list", "--help"}, {"--version"}} {
		if _, _, err := run(mk(), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{{"completion", "bash"}, {"__complete", "li"}, {"help", "list"}} {
		_, _, err := run(mk(), args...)
		t.Logf("%v -> err=%v", args, err)
		if !errors.Is(err, bad) {
			t.Fatalf("%v: expected the root hook to run and fail, got %v", args, err)
		}
	}
}
