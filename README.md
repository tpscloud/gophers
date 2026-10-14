# gophers

[![version](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Ftpscloud%2Fgophers%2Fmain%2F.tessl-plugin%2Fplugin.json&query=%24.version&label=version&prefix=v)](https://tessl.io/registry/tpscloud/gophers)
[![Skill quality](https://github.com/tpscloud/gophers/actions/workflows/skill-quality.yml/badge.svg)](https://github.com/tpscloud/gophers/actions/workflows/skill-quality.yml)
[![skills.sh](https://www.skills.sh/b/tpscloud/gophers)](https://www.skills.sh/tpscloud/gophers)
[![tessl](https://img.shields.io/endpoint?url=https%3A%2F%2Fapi.tessl.io%2Fv1%2Fbadges%2Ftpscloud%2Fgophers)](https://tessl.io/registry/tpscloud/gophers)

Skills for AI coding agents that work on Go programs:

- <a href="https://github.com/spf13/cobra" target="_blank" rel="noopener noreferrer">Cobra</a> and <a href="https://github.com/spf13/pflag" target="_blank" rel="noopener noreferrer">pflag</a> command-line interfaces
- <a href="https://github.com/spf13/viper" target="_blank" rel="noopener noreferrer">Viper</a> configuration
- <a href="https://github.com/wailsapp/wails" target="_blank" rel="noopener noreferrer">Wails</a> desktop apps, v2 and v3

The skills concentrate on the structural choices and library behavior that
cause real bugs rather than on syntax. Every library behavior a skill states
as fact has a test, and every Go example lives under `examples/`, built and
tested in CI against pinned module versions, so the guidance does not drift.

> [!NOTE]
> These are early versions of the skills. We have been dogfooding them for a
> few months. As CI gains more measures of how well each skill works, the
> results go into
> [SECURITY.md](https://github.com/tpscloud/gophers/blob/main/SECURITY.md)
> alongside the security and quality reports.

## Skills

These are third-party skills and are not affiliated with the projects they
document.
[SECURITY.md](https://github.com/tpscloud/gophers/blob/main/SECURITY.md) has the
independent security and quality reports.

**[cobra](https://github.com/tpscloud/gophers/blob/main/skills/cobra/SKILL.md)**: command trees, flags, hooks, completion, generated docs, and testable output and exit codes

**[viper](https://github.com/tpscloud/gophers/blob/main/skills/viper/SKILL.md)**: layered file, environment, and flag configuration, typed decoding, and reloads

**[wails2](https://github.com/tpscloud/gophers/blob/main/skills/wails2/SKILL.md)**: Wails v2 desktop apps: bound Go methods and generated bindings, the startup context and runtime calls, embedded assets, build tags, and Linux GTK3 and WebKitGTK dependencies

**[wails3](https://github.com/tpscloud/gophers/blob/main/skills/wails3/SKILL.md)**: Wails v3 desktop apps, a prerelease: services and generated bindings, errors as rejected promises, typed events, dialogs and windows, the wails3 CLI and Taskfile, server mode, and cgo with GTK4 and WebKitGTK 6.0 on Linux

The cobra and viper skills are separate on purpose: Cobra does not need Viper,
and neither skill adds it to a program that does not need layered
configuration.

## Install

Install with Vercel's <a href="https://skills.sh" target="_blank" rel="noopener noreferrer">skills CLI</a>:

```sh
npx skills add tpscloud/gophers
```

> [!NOTE]
> This is the recommended route. One command installs into Claude Code, Codex,
> and dozens of other agents from one shared copy, either per project (committed
> with your code) or for your user. It needs no clone, and `npx skills update`
> pulls new versions later.

It asks which skills and agents to install. Add `--skill cobra` (repeatable)
to pick skills, `-a claude-code` or `-a codex` to pick an agent, and `-g` to
install for your user instead of the current project. `--list` shows the
skills without installing.

### From a checkout

The skills also load straight from a clone of this repository. Replace
`/path/to/gophers` with its absolute path.

Claude Code, as a plugin for one session (skills are then named
`/gophers:cobra` and so on):

```sh
claude --plugin-dir /path/to/gophers
```

Claude Code or Codex, as personal skills that stay installed:

```sh
# Claude Code
mkdir -p "$HOME/.claude/skills"
ln -s /path/to/gophers/skills/* "$HOME/.claude/skills/"

# Codex
mkdir -p "$HOME/.agents/skills"
ln -s /path/to/gophers/skills/* "$HOME/.agents/skills/"
```

Start a new session afterwards. Symlinked skills follow the checkout, so keep
it in place.

### Examples

The Go programs the skills cite live in `examples/<skill>/`, one Go module per
skill so each keeps its own pinned versions. An installed skill links to them
on GitHub. To run one, clone the repository and work from its module, for
example `cd examples/cobra && go run ./cli/cmd/catalog list --json` or
`go test ./...`.

## Development

[AGENTS.md](https://github.com/tpscloud/gophers/blob/main/AGENTS.md) has the editing rules. CI uses
<a href="https://github.com/tpscloud/skillforge-ci" target="_blank" rel="noopener noreferrer">Skillforge</a>;
run `skillforge validate .` and `skillforge check .` before committing.

## License

MIT. See [LICENSE](https://github.com/tpscloud/gophers/blob/main/LICENSE). Each skill directory carries the same text in `LICENSES/MIT.txt` and a `NOTICE.md`, so a skill installed on its own keeps its terms. The documented libraries keep their own licenses and none is bundled. This project is independently maintained and is not affiliated with Cobra, Viper, or Wails.

Badges are maintained by this project and are not provider endorsements.
[SECURITY.md](https://github.com/tpscloud/gophers/blob/main/SECURITY.md) explains how to report a security issue.
