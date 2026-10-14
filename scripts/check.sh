#!/bin/sh
# Check the example modules and documentation. Skillforge validates frontmatter.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
# The examples need no C code. Wails v2 compiles cgo against GTK and WebKitGTK
# on Linux whenever cgo is on, and CI runners have only Go installed, so build
# everything without cgo unless the caller overrides it.
export CGO_ENABLED="${CGO_ENABLED:-0}"
fail=0

if ! command -v go >/dev/null 2>&1; then
  echo "error: Go is not on PATH; install the version required by examples/*/go.mod" >&2
  exit 1
fi

# Each skill has its own module under examples/<skill>/ so its pins stay
# independent. There is deliberately no go.work: a workspace would resolve one
# shared build list and move a skill off the versions it was verified against.
for mod in "$root"/examples/*/go.mod; do
  [ -f "$mod" ] || continue
  dir=$(dirname "$mod")
  name=${dir#"$root"/}
  echo "== $name: gofmt"
  unformatted=$(cd "$dir" && gofmt -l .)
  if [ -n "$unformatted" ]; then
    echo "$unformatted"
    echo "error: run gofmt -w on the files listed above" >&2
    fail=1
  fi
  echo "== $name: vet"
  if ! ( cd "$dir" && go vet ./... ); then
    echo "error: $name vet checks failed (see Go diagnostics above)" >&2
    fail=1
  fi
  echo "== $name: test"
  if ! ( cd "$dir" && go test ./... ); then
    echo "error: $name tests failed (see Go diagnostics above)" >&2
    fail=1
  fi
done

echo "== sweeping markdown for em and en dashes"
if grep -rn --include='*.md' --exclude-dir=.claude --exclude-dir=.git -e "$(printf '\342\200\224')" -e "$(printf '\342\200\223')" "$root" | grep -v '/\.skill-quality/'; then
  echo "error: prohibited em or en dash in the files listed above" >&2; fail=1
fi

echo "== checking Go fences name their example file or are marked as excerpts"
# The first line of every Go fence in the skill or a reference is either
#   // examples/<path>.go        (trimmed from that file)
#   // excerpt ...               (API sketch, no example file)
# and a named example file must exist.
for f in "$root"/skills/*/SKILL.md "$root"/skills/*/references/*.md; do
  [ -f "$f" ] || continue
  awk -v root="$root" '
    /^```go/ {
      if ((getline line) <= 0) { printf "%s:%d: unterminated Go fence\n", FILENAME, NR; bad = 1; exit }
      if (line ~ /^\/\/ examples\/[^ ]+\.go( |$)/) {
        p = line; sub(/^\/\/ /, "", p); sub(/ .*/, "", p)
        # Open the file instead of shelling out, so a crafted path cannot run a command.
        f = root "/" p
        ok = ((getline tmp < f) >= 0); close(f)
        if (!ok) {
          printf "%s:%d: no such example file %s\n", FILENAME, NR, p; bad = 1
        }
      } else if (line !~ /^\/\/ excerpt/) {
        printf "%s:%d: Go fence must start with // examples/<path> or // excerpt\n", FILENAME, NR; bad = 1
      }
    }
    END { exit bad }' "$f" || fail=1
done

if [ "$fail" -ne 0 ]; then
  echo "Repository checks failed. Fix the reported errors and rerun skillforge check ." >&2
  exit 1
fi
echo "All repository checks passed."
