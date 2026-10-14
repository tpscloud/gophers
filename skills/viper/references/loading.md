# Loader, file resolution, and tests

This pattern targets Go 1.26 and Viper v1.21.0. Each Go block is trimmed from
the repository's
[examples module](https://github.com/tpscloud/gophers/tree/main/examples/viper),
which CI builds, vets, and tests. Adapt the prefix, keys, file name, and
validation together; they are one schema.

## Schema, defaults, and flags

The struct, the defaults, the env-only keys, and the flag-to-key map live
next to each other, so adding a setting touches one file:

```go
// examples/viper/config/config.go (trimmed: Load, resolve, and Validate shown separately)
type Config struct {
	Server Server `mapstructure:"server"`
	Log    Log    `mapstructure:"log"`
	Token  string `mapstructure:"token"` // env or file only; never written back
}

type Server struct {
	Addr        string        `mapstructure:"addr"`
	Port        int           `mapstructure:"port"`
	ReadTimeout time.Duration `mapstructure:"read-timeout"`
	TLS         bool          `mapstructure:"tls"`
}

type Source struct {
	File  string         // explicit --config path; it must exist
	Dirs  []string       // searched in order for FileName; a miss is fine
	Flags *pflag.FlagSet // already parsed; only flags from AddFlags bind
}

var flagKeys = map[string]string{
	"addr":      "server.addr",
	"port":      "server.port",
	"tls":       "server.tls",
	"log-level": "log.level",
}

func AddFlags(fs *pflag.FlagSet) {
	fs.String("addr", defaults["server.addr"].(string), "listen address")
	fs.Int("port", defaults["server.port"].(int), "listen port")
	fs.Bool("tls", defaults["server.tls"].(bool), "serve TLS (needs APP_TOKEN)")
	fs.String("log-level", defaults["log.level"].(string), "debug, info, warn, or error")
}

var defaults = map[string]any{
	"server.addr":         "127.0.0.1",
	"server.port":         8080,
	"server.read-timeout": "5s",
	"server.tls":          false,
	"log.level":           "info",
}

// Keys without a default still need an environment binding, or Unmarshal
// never sees them.
var envOnly = []string{"token"}
```

Flag defaults match `SetDefault` so `--help` shows the real default. Since
every bound key has a `SetDefault`, an unchanged flag never decides a value.

## Choosing the file

The loader checks one exact name per directory instead of using
`SetConfigName`, which would also find `app.json` or `app.toml` and parse them
by extension (or as YAML, if `SetConfigType("yaml")` is set):

```go
// examples/viper/config/config.go (trimmed: resolve only)
func resolve(src Source) (string, error) {
	if src.File != "" {
		return src.File, nil
	}
	for _, dir := range src.Dirs {
		path := filepath.Join(dir, FileName)
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			return path, nil
		case err == nil, errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return "", err // permission denied and the like are not a miss
		}
	}
	return "", nil
}
```

An explicit path is returned unchecked, so a missing `--config` fails in
`ReadInConfig` with an `fs.ErrNotExist` error. The first directory with the
file wins; files are not merged. Which directories to search is the
application's choice. The example's `main` passes the user config directory
and then the working directory.

If a project keeps Viper's own discovery instead, ignore only
`viper.ConfigFileNotFoundError` and drop `SetConfigType` unless every
candidate file really is that format:

```go
// excerpt: discovery with an optional file
v.SetConfigName("app")
v.AddConfigPath(dir)
if err := v.ReadInConfig(); err != nil {
	if _, missing := errors.AsType[viper.ConfigFileNotFoundError](err); !missing {
		return Config{}, err
	}
}
```

## Validation

Validate after decoding and report every problem at once:

```go
// examples/viper/config/config.go (trimmed: Validate only)
func (c Config) Validate() error {
	var errs []error
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port %d is outside 1-65535", c.Server.Port))
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, fmt.Errorf("server.read-timeout must be positive, got %s", c.Server.ReadTimeout))
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("log.level %q is not one of debug, info, warn, error", c.Log.Level))
	}
	if c.Server.TLS && c.Token == "" {
		errs = append(errs, errors.New("server.tls needs a token (set APP_TOKEN)"))
	}
	return errors.Join(errs...)
}
```

## Cobra wiring

The root owns `--config`; the command that needs settings registers the
schema flags and loads in its `RunE`. Completion and help never touch the
file:

```go
// examples/viper/cli/cli.go (trimmed: package clause and Serve type omitted)
func NewRoot(dirs []string, serve Serve) *cobra.Command {
	root := &cobra.Command{
		Use:           "app",
		Short:         "Serve the app",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	var file string
	root.PersistentFlags().StringVar(&file, "config", "", "config file (default: the first app.yaml in the search directories)")

	srv := &cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		Args:  cobra.NoArgs,
		// Load here, not in a root PersistentPreRunE: Cobra runs the root
		// hook for `completion`, `__complete`, and `help <command>` too.
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(config.Source{File: file, Dirs: dirs, Flags: cmd.Flags()})
			if err != nil {
				return err
			}
			return serve(cmd.Context(), cfg)
		},
	}
	config.AddFlags(srv.Flags())
	if err := srv.RegisterFlagCompletionFunc("log-level", cobra.FixedCompletions(
		[]string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp,
	)); err != nil {
		panic(err) // programming error: AddFlags registers log-level
	}
	root.AddCommand(srv)
	return root
}
```

If several commands need settings, call one helper from each `RunE`, or keep a
root hook that returns early for commands that do not need config. Do not
make every command pay for a file only some of them use.

## Test matrix

`examples/viper/config/config_test.go` and `examples/viper/cli/cli_test.go`
cover these rows. Each test clears every `APP_*` variable first.

| Inputs | Expected |
| --- | --- |
| Nothing set | the defaults |
| File `port: 9000`, flag unchanged | 9000 |
| Same plus `APP_SERVER_PORT=9100` | 9100 |
| Same plus `--port=9200` | 9200 |
| File `tls: true`, `--tls=false` | false |
| `APP_TOKEN` only, no default | decoded into `Token` |
| `APP_SERVER_READ_TIMEOUT=750ms` | 750ms (hyphen mapped to `_`) |
| `app.yaml` in two search directories | the first one, not merged |
| `app.json` and `app.toml` beside `app.yaml` | only `app.yaml` read |
| `--config` path missing | error wrapping `fs.ErrNotExist` |
| Malformed YAML | error naming the file |
| Unknown key `prot` | decode error |
| `APP_SERVER_PORT=eighty` | decode error |
| Port 70000, timeout `0s`, level `loud` | validation errors |
| `--json` registered beside the schema flags | not bound, no error |
| `__complete`, `completion zsh`, `help serve` with a broken file | succeed |
| `serve` with a broken file | error, service not called |
