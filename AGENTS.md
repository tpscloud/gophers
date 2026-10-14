# gophers

Agent skills for Go, written from the module source rather than from memory.
Every Go block in a skill is trimmed from a file under `examples/` that must
build and pass its tests. If the two ever disagree, the example is right and
the skill text is wrong.

## What is in here

```
skills/<name>/SKILL.md        the skill (frontmatter plus body)
skills/<name>/references/     deep dives, loaded on demand
skills/<name>/agents/         Codex display metadata
examples/<name>/              one Go module per skill: examples and behavior tests
scripts/check.sh              gofmt, vet, and tests per module; prose and fence sweeps
evals/<name>-<scenario>/      Tessl eval scenarios
.claude-plugin/plugin.json    lets the repo install as a Claude Code plugin
.tessl-plugin/plugin.json     Tessl registry manifest
skills.sh.json                skill groupings for the skills.sh page
```

| Skill  | Covers                                    | Pins (in `examples/<skill>/go.mod`) |
|--------|-------------------------------------------|-------------------------------------|
| cobra  | github.com/spf13/cobra, spf13/pflag       | cobra v1.10.2, pflag v1.0.9         |
| viper  | github.com/spf13/viper, spf13/pflag       | viper v1.21.0, pflag v1.0.10        |
| wails2 | github.com/wailsapp/wails/v2 desktop apps | wails v2.16.0                       |
| wails3 | github.com/wailsapp/wails/v3 desktop apps | wails v3.0.0-beta.29 (prerelease)   |

Each skill has its own module so its pins stay independent. Do not add a
`go.work`: a workspace resolves one shared build list and would move a skill
off the versions it was verified against (Viper requires a newer pflag than
the cobra skill pins). Bump a pin in its module first, run the check, fix what
fails in its behavior tests, and only then edit prose.

## Rules for editing a skill

1. Verify before you write. Use `go doc` against the pinned module, or read the
   module source in the Go module cache. Do not trust a README, a blog post, or
   another skill.
2. A statement about library behavior needs a test in
   `examples/<skill>/behavior/` or a cited source line.
3. Every Go block is either trimmed from a file under `examples/` or a short
   API sketch. Its first line says which: `// examples/<path>.go` (the check
   verifies the file exists) or `// excerpt: ...`. When a trimmed block and
   its file disagree, fix the block.
4. Examples follow modern Go for the `go` directive in their module. Keep each
   skill a reusable reference, not tied to one application.
5. Frontmatter carries only `name`, `description`, and `license`. Double quote
   the description, keep it under 1024 characters, and say when to use the
   skill and when not to.
6. Keep SKILL.md short: a summary, an ordered workflow that ends in a
   verification step, the rules and pitfalls, and version pins once in a final
   Sources section. Put per-topic detail in `references/`, one level deep,
   each linked from SKILL.md with a line saying what it holds.
7. Each skill directory carries `LICENSES/MIT.txt` and a `NOTICE.md`, so a
   skill installed on its own keeps its terms.
8. No em dashes or en dashes anywhere: prose, code comments, commit messages.
9. Write plainly. Say what the API does and what goes wrong when it is misused.

## Checking your work

```sh
skillforge validate .
skillforge check .
```

`check` runs `scripts/check.sh`, which needs Go. Run both before every commit.
Install Skillforge from its
[releases](https://github.com/tpscloud/skillforge-ci/releases) or build it from
`.skill-quality/` (`go -C .skill-quality build -o /tmp/skillforge .`). CI builds
it from that vendored copy. Do not edit `.skill-quality/` by hand; update it
with `skillforge install . --update` from a newer Skillforge release.

## Commits

One skill per commit where possible. Start the subject with the skill name,
for example `viper: test env key replacement`.

## Adding a skill

1. Write `skills/<name>/SKILL.md`, `LICENSES/MIT.txt`, and `NOTICE.md`.
2. Create `examples/<name>/` as its own module with examples and behavior tests.
3. Add the skill to `.skill-quality.json`, `skills.sh.json`,
   `.tessl-plugin/plugin.json`, the table above, README.md, and SECURITY.md.
4. Add eval scenarios under `evals/<name>-<scenario>/`.
5. Run validation and the checks, then commit.

## Publishing to Tessl

The "Publish to Tessl" workflow runs on pushes to `main` that change `skills/`,
`evals/`, or `.tessl-plugin/plugin.json`, and publishes only when the version in
`.tessl-plugin/plugin.json` is not in the registry yet and the `TESSL_TOKEN`
repository secret is set. Publishing runs every eval scenario, which costs
Tessl credits, so bump the version (and the one in `.claude-plugin/plugin.json`)
only for a release. Documentation changes never publish.
