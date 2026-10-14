# Command construction and tests

This pattern targets Go 1.26 and Cobra v1.10.2. Each Go block is trimmed from a
file in the repository's
[examples module](https://github.com/tpscloud/gophers/tree/main/examples/cobra/cli),
which CI builds, vets, and tests. Adapt types and exit-code policy to the
application; `List` stands for a shared core service and `Tags` for a fast,
read-only source of completion candidates.

```go
// examples/cobra/cli/cli.go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type List func(context.Context, string) ([]string, error)

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
```

## Process boundary

`main` only calls `os.Exit`. A `run` function owns signal handling, deferred
cleanup, the single error report on stderr, and the mapping from errors to the
application's documented exit codes:

```go
// examples/cobra/cli/cmd/catalog/main.go
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

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
```

`version` is a package variable set at build time with
`go build -ldflags "-X main.version=1.2.3"`. Setting `root.Version` adds a
`--version` flag that prints `catalog version 1.2.3` to the out writer;
`SetVersionTemplate` changes the format.

Passing `os.Args[1:]` explicitly keeps `run` testable. A nil argument slice
makes Cobra fall back to the process's own arguments, so tests that mean "no
arguments" pass `[]string{}`.

For a combined CLI/TUI, give the root `Args: cobra.NoArgs` and a `RunE` that
launches the TUI only when appropriate. Detect non-TTY execution and explicit
machine-readable mode before starting it; subcommands still use the same core.

## Structure beyond one command

- Help groups: set `GroupID` on commands and register the group with
  `root.AddGroup(&cobra.Group{ID: "core", Title: "Core commands:"})`. Order
  does not matter because groups are resolved at execution, but an unknown
  `GroupID` panics then, so test `--help`.
- `Hidden: true` removes a command from help and it still runs.
  `Deprecated: "use X instead"` also hides it and prints a deprecation notice
  each time it runs.
- `MarkFlagsRequiredTogether` and `MarkFlagsOneRequired` sit beside
  `MarkFlagsMutuallyExclusive`. Like required flags, they only count flags the
  user typed; a default value does not satisfy them.
- Customize help with `SetHelpTemplate`, `SetUsageTemplate`, or
  `SetHelpFunc` on the root. Test the parts you changed rather than the whole
  page; if exact output matters, compare against a golden file you regenerate
  on purpose.

## Tests

Build a fresh root in every test. This one checks that the context and flag
reach the service, the payload is valid JSON on stdout, and stderr stays empty:

```go
// examples/cobra/cli/cli_test.go
func TestListJSON(t *testing.T) {
	var out, diagnostics bytes.Buffer
	root := NewRoot(func(ctx context.Context, tag string) ([]string, error) {
		if ctx != t.Context() || tag != "go" {
			t.Fatalf("context or tag not forwarded: %q", tag)
		}
		return []string{"example"}, nil
	}, noTags)
	root.SetArgs([]string{"list", "--tag", "go", "--json"})
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal(out.Bytes(), &names); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"example"}) || diagnostics.Len() != 0 {
		t.Fatalf("payload=%q diagnostics=%q", out.String(), diagnostics.String())
	}
}
```

The same file covers the other cases worth copying:

- invalid arguments and unknown flags return an error, never call the service,
  and print nothing (the root silences errors and usage);
- a service error comes back unchanged, so callers can use `errors.Is`;
- a cancelled context reaches the service;
- a failing stdout writer surfaces as an error;
- an empty result encodes as `[]`, not `null`;
- `__complete list --tag g` returns the matching tags and directive `:4`
  (`ShellCompDirectiveNoFileComp`) without running the command.

For many argument cases, use a table of `args`, expected error, and expected
output, building a fresh root in each subtest. Always execute the root:
`child.Execute()` also runs from the root of the tree, with the root's
arguments. Context cancellation only works if the core operation observes it. Test the
printed error and exit code separately, at the process boundary, as
`examples/cobra/cli/cmd/catalog/main_test.go` does.
