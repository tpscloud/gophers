# A list command with JSON output

A Go CLI uses Cobra v1.10.2. Add a `list` subcommand that prints project names
from an injected `func(context.Context) ([]string, error)` service, one per
line, or as a JSON array with `--json`. Scripts will run
`app list --json 2>/dev/null | jq .`, so the JSON must reach stdout in the real
binary, not only in tests.

Return the command constructor and a Go test for the JSON case.
