---
name: cobra
description: "Build, extend, debug, and test Go command-line interfaces with spf13/cobra and pflag: command trees and subcommands, flags, argument validation, hooks, shell completion, generated docs, context cancellation, and stdout, stderr, and exit code contracts, including a CLI that sits beside a TUI. Use when writing or changing a Go CLI that uses Cobra, adding a subcommand or flag, or debugging flag inheritance, hook order, completion, or output that lands on the wrong stream. Not for layered file, environment, and flag configuration (use the viper skill) or for CLIs in other languages."
license: "MIT"
---

# Cobra

Build Go command-line programs with Cobra so they stay testable and
predictable: fresh command trees, thin handlers over shared operations,
explicit argument validation, payloads on stdout, diagnostics on stderr, and
one place that reports errors. The pitfalls below are the lifecycle and flag
behaviors that cause real bugs.

## Workflow

1. Read the project's `go.mod`. Use only APIs available in its Cobra version;
   do not upgrade a dependency just to match this skill. Write Go for the
   version in its `go` directive (if a modern-Go guidelines skill such as
   `use-modern-go` is installed, follow it for that version).
2. Keep an existing working CLI. Migrate to or from Cobra only when asked or
   when the task needs it.
3. Build or change the command tree following
   [Implementation choices](#implementation-choices). For a new command, start
   from the constructor and test pattern in
   [references/commands.md](references/commands.md).
4. Check the change against [Flags and lifecycle pitfalls](#flags-and-lifecycle-pitfalls).
5. Before finishing, run the checks in [Verify behavior](#verify-behavior).

## Implementation choices

A new command starts from a constructor that takes its services and returns a
fresh tree:

```go
// examples/cobra/cli/cli.go (trimmed: text output, empty results, and completion omitted)
func NewRoot(list List, tags Tags) *cobra.Command {
	root := &cobra.Command{
		Use:           "catalog",
		SilenceErrors: true, // the process boundary owns error reporting
		SilenceUsage:  true, // explicit --help still works
	}
	var tag string
	ls := &cobra.Command{
		Use:  "list",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := list(cmd.Context(), tag)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(names)
		},
	}
	ls.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	root.AddCommand(ls)
	return root
}
```

A test builds its own tree, sets `SetArgs`, `SetOut`, and `SetErr`, runs
`ExecuteContext(t.Context())`, and asserts on the captured buffers. The full
version, with text output, completion, and tests, is in
[references/commands.md](references/commands.md).

- Construct fresh command trees with `NewRoot(...) *cobra.Command` (or an
  error-returning constructor if setup can fail). Inject services and writers;
  avoid package-level commands, mutable options, and registration in `init()`.
  Fresh instances prevent flags and tests from leaking state between executions.
- Keep handlers thin. A CLI and a TUI over the same program should call the
  same core operations.
- Put operations in `RunE`, with explicit `Args` validation (`NoArgs`, `ExactArgs`,
  `MinimumNArgs`, etc.). Without `Args`, a leaf command accepts arbitrary
  arguments, and a nested command group that has no `RunE` prints help for
  `app group typo` and exits 0. A script cannot tell that from success.
- Use `Flags()` for a command's own options and `PersistentFlags()` for options
  genuinely shared by descendants. Parent local flags are not inherited.
- Use `ExecuteContext(ctx)` and pass `cmd.Context()` into core operations.
  Define signal handling and exit codes at the process boundary. Cobra does not
  choose your cancellation or partial-failure exit codes.
- Write payloads through `cmd.OutOrStdout()` and diagnostics through
  `cmd.ErrOrStderr()`. Keep JSON output free of progress, styling, and prose.
  Do not use `cmd.Print*` for payloads: it writes to the `SetOut` writer when
  one is set and otherwise to the process's stderr, so a test that calls
  `SetOut` passes while the real binary sends the payload to stderr.
- Assign error printing to one layer. If the caller prints errors, set
  `SilenceErrors: true`; use `SilenceUsage` deliberately so runtime failures do
  not emit a help page. Still return the error. Do not `os.Exit`, `log.Fatal`, or
  `cobra.CheckErr` inside reusable commands.
- Create dependencies only when the operation needs them. `--help`, `--version`,
  and completion should not require a valid config, network login, or a writable
  state database merely because the command tree was constructed.

## Flags and lifecycle pitfalls

- Required flags and flag groups concern flags explicitly supplied by the user
  (`Changed`), not whether an effective value is nonzero. If config or env can
  satisfy a required setting, validate the resolved setting instead.
- Distinguish `--enabled=false` from an omitted flag using `Changed`. Prefer
  `StringArray` for repeated literal values, `StringSlice` for CSV/repeated
  values, and `Count` for `-v`, `-vv`, etc.
- Check errors from flag marking and completion registration. Group helpers such
  as `MarkFlagsMutuallyExclusive` can panic when given an undefined flag name;
  register flags first.
- Persistent hooks are **nearest ancestor wins** by default, including hooks on
  the command itself. A child's persistent hook can bypass root configuration
  loading. Prefer explicit shared setup over surprising hook inheritance.
- A root `PersistentPreRunE` is skipped for `--help` and `--version`, but it
  runs for `completion <shell>`, every `__complete` request, and
  `help <command>`. If it loads config and fails, tab completion breaks
  wherever the config file is missing or invalid. Load config inside the
  operation that needs it, or return early from the hook for those commands.
- `cobra.EnableTraverseRunHooks` is a process-global opt-in: persistent pre-hooks
  then run root-to-leaf and post-hooks leaf-to-root. Do not toggle it per command
  or in parallel tests. `TraverseChildren` is a different flag-parsing option.
- `Args` validation runs first, then pre-hooks, then required-flag and
  flag-group validation. Keep mutations in `RunE`. Post-hooks do not run after a
  failed `RunE`; acquire resources and `defer` cleanup in the same operation
  instead.
- When combining Viper, bind flags after defining them, then read the effective
  values from Viper after parsing. Viper does not update variables bound with
  `StringVar` from config or environment values.

## Task-specific references

- [references/commands.md](references/commands.md): the default starting
  pattern. A constructor with injected services, context, output writers, the
  process boundary, and command tests.
- [references/completion.md](references/completion.md): shell completion,
  generated docs, and a checklist for debugging routing and flags.
- For file/env/flag precedence and decoding, use the `viper` skill if it is
  installed. Without it, consult the upstream Viper API; do not add Viper merely
  because a command uses Cobra.

[references/commands.md](references/commands.md) also covers `Version`, help
groups, hidden and deprecated commands, and flag groups. For `MatchAll`, custom
`pflag.Value` types, and the rest of the API, see the API documentation under
[Sources](#sources).

## Verify behavior

Before finishing, test with fresh command instances using `SetArgs`, `SetIn`,
`SetOut`, `SetErr`, and `ExecuteContext`, and check:

- invalid input does not invoke the service;
- output lands on the right stream;
- cancellation reaches the service;
- explicit false and zero values work;
- ancestor options and config behave on leaf commands;
- custom completion returns the expected candidates and directive.

Avoid snapshots of entire generated help pages unless their exact layout is a
requirement. Run `go vet` and `go test` for the packages you changed. When a
check fails, find the matching entry in
[Flags and lifecycle pitfalls](#flags-and-lifecycle-pitfalls), fix the
command, and rerun every check.

## Sources

Verified against cobra v1.10.2 and pflag v1.0.9; each stated behavior has a test
in the [examples module](https://github.com/tpscloud/gophers/tree/main/examples/cobra).
When the project's version differs, trust `go doc github.com/spf13/cobra.Command`
and its module source over this skill.

- [command.go (execution and hooks)](https://github.com/spf13/cobra/blob/v1.10.2/command.go)
- [User guide](https://github.com/spf13/cobra/blob/v1.10.2/site/content/user_guide.md)
- [pflag](https://github.com/spf13/pflag/tree/v1.0.9)
- [API documentation](https://pkg.go.dev/github.com/spf13/cobra@v1.10.2)
