---
name: viper
description: "Load, debug, and test layered Go configuration with spf13/viper: defaults, YAML, JSON, or TOML config files, environment variables with a prefix, and Cobra or pflag flags resolved in a fixed precedence order, then decoded into a typed struct, validated, and optionally reloaded or written back. Use when a Go program already uses Viper, when its settings must come from more than one of file, environment, and flags, or when a config value, env var, or flag is ignored or decodes wrong. Not for command trees and flag definitions alone (use the cobra skill), for projects on another config library such as koanf or envconfig, or for application state that changes at runtime."
license: "MIT"
---

# Viper

Load settings with Viper so they stay predictable and testable: a fresh
instance per load, every key registered explicitly, one file chosen on
purpose, a typed and validated snapshot handed to the rest of the program,
and no config loading on paths that do not need it. The pitfalls below are
the precedence, environment, file, and decoding behaviors that cause real
bugs.

## Workflow

1. Read the project's `go.mod`. Use only APIs available in its Viper version;
   do not upgrade a dependency just to match this skill. Write Go for the
   version in its `go` directive (if a modern-Go guidelines skill such as
   `use-modern-go` is installed, follow it for that version).
2. Decide whether Viper belongs. Keep a project's existing config library
   (koanf, envconfig, the standard library) in place. A CLI that only needs
   flags does not need Viper, and mutable state such as caches or inventories
   belongs in the application's storage, not in config.
3. Write or fix the loader following [Loader pattern](#loader-pattern), then
   check it against [Precedence and environment](#precedence-and-environment)
   and [Files and decoding](#files-and-decoding).
4. With Cobra, load config inside the commands that need it, as in
   [With Cobra](#with-cobra).
5. Before finishing, run the checks in [Verify behavior](#verify-behavior).

## Loader pattern

One function builds a new instance, registers every layer, reads at most one
file, decodes, and validates:

```go
// examples/viper/config/config.go (trimmed: types, AddFlags, and resolve omitted)
func Load(src Source) (Config, error) {
	v := viper.New()
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	for key := range defaults {
		if err := v.BindEnv(key); err != nil {
			return Config{}, err
		}
	}
	for _, key := range envOnly {
		if err := v.BindEnv(key); err != nil {
			return Config{}, err
		}
	}
	if src.Flags != nil {
		for name, key := range flagKeys {
			if f := src.Flags.Lookup(name); f != nil {
				if err := v.BindPFlag(key, f); err != nil {
					return Config{}, err
				}
			}
		}
	}
	path, err := resolve(src)
	if err != nil {
		return Config{}, err
	}
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read %s: %w", path, err)
		}
	}
	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

- Use `viper.New()` per load or command tree, not the package-level
  singleton. Separate instances share nothing, so tests do not leak state.
- Pass the decoded `Config` to the core. Keep `*viper.Viper` out of business
  logic.
- Bind only flags that belong to the schema. With `UnmarshalExact`, an
  operational flag such as `--json` bound through `BindPFlags` becomes an
  unknown key.

The full loader, its file resolution, and its test matrix are in
[references/loading.md](references/loading.md).

## Precedence and environment

Highest first, as `Get` and `Unmarshal` resolve a key: `Set`, a flag the user
changed, environment, config file, `SetDefault`, then an unchanged flag's own
default.

- Put defaults in `SetDefault`. Copying parsed flag values into `Set` makes an
  omitted flag's default beat env and the file.
- An explicit `--tls=false` is a changed flag and wins over a `true` default
  or file value. Never decide "unset" with `value == zero`.
- Viper does not write env or file values back into variables bound with
  `StringVar`, `IntVar`, and so on. Read effective values from the decoded
  struct after parsing.
- `BindPFlag` returns an error for a nil flag, which is what `Lookup` returns
  for a misspelled name. Check it.
- `AutomaticEnv` answers `Get` for any key but adds nothing to `AllKeys`, so
  `Unmarshal` misses an env-only key. Give every decoded key a `SetDefault`
  or a `BindEnv`.
- `BindEnv("server.read-timeout")` derives `PREFIX_SERVER.READ-TIMEOUT` in
  upper case, and `SetEnvKeyReplacer` then turns it into
  `APP_SERVER_READ_TIMEOUT`. `BindEnv(key, "DATABASE_PASSWORD")` uses that
  name exactly, without the prefix.
- An empty variable counts as unset unless `AllowEmptyEnv(true)`.
- Keys are case-insensitive, and map keys read from a file are lowercased
  (`labels: {Team: Core}` decodes as `team`). Keep case-sensitive data in a
  list of structs, not in map keys.

## Files and decoding

- Discovery with `SetConfigName` and `AddConfigPath` tries extensions in the
  order of `viper.SupportedExts`, so `app.json` wins over `app.yaml` in the
  same directory. `SetConfigType("yaml")` does not filter that search; it
  forces the YAML parser on whatever was found, including `app.toml`. For a
  single-format app, resolve the exact file yourself and call `SetConfigFile`.
- A discovery miss returns `viper.ConfigFileNotFoundError`; ignore only that.
  A missing `SetConfigFile` path returns a plain `fs.ErrNotExist` error, so a
  user-supplied `--config` that does not exist still fails, as it should.
  Match it as a value type, with `errors.AsType` or with `errors.As` and a
  `viper.ConfigFileNotFoundError` variable; a pointer target never matches.
- An explicit file with no extension needs `SetConfigType`, or it fails with
  `UnsupportedConfigError`. `SetConfigType` also overrides an explicit file's
  extension.
- The core decodes YAML, JSON, TOML, and dotenv only. `.ini`, `.hcl`, and
  `.properties` fail with "decoder not found for this format"; those codecs
  live in `github.com/go-viper/encoding` and need a codec registry.
- `Unmarshal` ignores unknown keys; `UnmarshalExact` rejects them, which
  catches typos in a closed schema.
- `GetInt` returns 0 for `eighty`; `Unmarshal` reports the error. Decode, do
  not trust getters, to detect bad input.
- From an env string, `Unmarshal` splits a slice on commas only
  (`a,b c` gives `a` and `b c`) while `GetStringSlice` splits on whitespace
  (`a,b` and `c`).
- `viper.DecodeHook` replaces the default hooks (duration and comma-slice
  parsing). Compose them back with
  `mapstructure.ComposeDecodeHookFunc` from `github.com/go-viper/mapstructure/v2`.
- Decoding is not validation. Check ranges, enums, and combinations after
  `Unmarshal`, and report them together.

## With Cobra

Cobra runs a root `PersistentPreRunE` for `completion`, every hidden
`__complete` request, and `help <command>`. Loading config there breaks tab
completion on a machine whose config is missing or broken. Load in the
`RunE` of each command that needs settings, after Cobra has parsed flags:

```go
// examples/viper/cli/cli.go (trimmed: root flags and completion omitted)
RunE: func(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(config.Source{File: file, Dirs: dirs, Flags: cmd.Flags()})
	if err != nil {
		return err
	}
	return serve(cmd.Context(), cfg)
},
```

`MarkFlagRequired` checks only that a flag was typed. When env or a file can
supply the setting, validate the decoded value instead. For command trees,
hooks, and output streams, use the
[cobra skill](https://github.com/tpscloud/gophers/tree/main/skills/cobra)
if it is installed.

## References

- [references/loading.md](references/loading.md): the complete loader, file
  resolution, Cobra wiring, and the test matrix to copy.
- [references/reload-and-write.md](references/reload-and-write.md):
  `WatchConfig` semantics and a safe reload pattern, writing config files,
  `Sub`, and answering "where did this value come from".

## Verify behavior

Before finishing, test the loader with a fresh instance, `t.TempDir()` files,
and `t.Setenv` for every variable it reads (cleared first, since ambient
values on a CI runner change results; do not use `t.Parallel` with
`t.Setenv`). Cover:

- each precedence step: default, file, env, changed flag;
- an explicit false or zero flag;
- an env-only key reaching the decoded struct;
- an optional file that is absent, and a `--config` path that is absent;
- a malformed file, an unknown key, and a malformed number;
- each validation rule;
- completion and help with a broken config, if the program uses Cobra.

Run `go vet` and `go test` for the packages you changed. When a check fails,
find the matching entry in [Precedence and environment](#precedence-and-environment)
or [Files and decoding](#files-and-decoding), fix the loader, and rerun every
check.

## Sources

Verified against viper v1.21.0 with pflag v1.0.10 and cobra v1.10.2; each
stated behavior has a test in the
[examples module](https://github.com/tpscloud/gophers/tree/main/examples/viper).
When the project's version differs, trust `go doc github.com/spf13/viper` and
its module source over this skill, and recheck the error types after any
upgrade.

- [viper.go (resolution, decoding, watching, writing)](https://github.com/spf13/viper/blob/v1.21.0/viper.go)
- [file.go (config discovery)](https://github.com/spf13/viper/blob/v1.21.0/file.go)
- [UPGRADE.md (codecs and finders since v1.20)](https://github.com/spf13/viper/blob/v1.21.0/UPGRADE.md)
- [API documentation](https://pkg.go.dev/github.com/spf13/viper@v1.21.0)
