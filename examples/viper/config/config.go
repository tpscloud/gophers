// Package config loads layered settings with Viper: defaults, an optional
// YAML file, APP_* environment variables, and parsed command-line flags,
// decoded into a typed, validated snapshot.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the snapshot handed to the rest of the program. Nothing outside
// this package sees a *viper.Viper.
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

type Log struct {
	Level string `mapstructure:"level"`
}

// FileName is the only file the loader looks for in each search directory.
const FileName = "app.yaml"

// EnvPrefix makes server.read-timeout readable from APP_SERVER_READ_TIMEOUT.
const EnvPrefix = "APP"

// Source says where settings may come from. The zero value means defaults and
// environment only.
type Source struct {
	File  string         // explicit --config path; it must exist
	Dirs  []string       // searched in order for FileName; a miss is fine
	Flags *pflag.FlagSet // already parsed; only flags from AddFlags bind
}

// flagKeys maps flag names to configuration keys. Only these flags bind, so
// operational flags such as --json never reach UnmarshalExact.
var flagKeys = map[string]string{
	"addr":      "server.addr",
	"port":      "server.port",
	"tls":       "server.tls",
	"log-level": "log.level",
}

// AddFlags registers the configuration flags on a command's flag set. Their
// defaults match SetDefault so help text tells the truth. Because every bound
// key has a SetDefault, Viper ignores a flag the user did not set.
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

// Load builds a fresh Viper instance, resolves every layer, and returns a
// validated snapshot. Call it after flag parsing.
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

// resolve picks the file to read. An explicit path is used as given, so a
// missing one fails in ReadInConfig. Otherwise the first FileName found in
// Dirs wins. Checking one exact name avoids Viper's extension search, which
// would pick up app.json or app.toml from the same directory.
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

// Validate checks what decoding cannot: ranges, enums, and combinations.
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
