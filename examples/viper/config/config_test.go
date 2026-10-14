package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

var appEnv = []string{
	"APP_SERVER_ADDR", "APP_SERVER_PORT", "APP_SERVER_READ_TIMEOUT",
	"APP_SERVER_TLS", "APP_LOG_LEVEL", "APP_TOKEN",
}

// isolate clears every variable the loader reads, so ambient values on a
// developer machine or CI runner cannot change a result. t.Setenv restores
// them afterwards and marks the test as not parallel.
func isolate(t *testing.T) {
	t.Helper()
	for _, name := range appEnv {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func flags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(fs)
	fs.Bool("json", false, "an operational flag that must not bind")
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestDefaults(t *testing.T) {
	isolate(t)
	cfg, err := Load(Source{Dirs: []string{t.TempDir()}, Flags: flags(t)})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Server: Server{Addr: "127.0.0.1", Port: 8080, ReadTimeout: 5 * time.Second},
		Log:    Log{Level: "info"},
	}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, FileName, "server:\n  port: 9000\n")
	cases := []struct {
		name string
		env  string
		args []string
		want int
	}{
		{"file beats default and unchanged flag", "", nil, 9000},
		{"env beats file", "9100", nil, 9100},
		{"changed flag beats env", "9100", []string{"--port=9200"}, 9200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolate(t)
			if c.env != "" {
				t.Setenv("APP_SERVER_PORT", c.env)
			}
			cfg, err := Load(Source{Dirs: []string{dir}, Flags: flags(t, c.args...)})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Server.Port != c.want {
				t.Fatalf("port=%d, want %d", cfg.Server.Port, c.want)
			}
		})
	}
}

func TestExplicitFalseFlagBeatsFile(t *testing.T) {
	isolate(t)
	t.Setenv("APP_TOKEN", "secret")
	dir := t.TempDir()
	write(t, dir, FileName, "server:\n  tls: true\n")
	cfg, err := Load(Source{Dirs: []string{dir}, Flags: flags(t, "--tls=false")})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.TLS {
		t.Fatal("--tls=false was ignored")
	}
}

func TestEnvOnlyKeyAndHyphenatedKey(t *testing.T) {
	isolate(t)
	t.Setenv("APP_TOKEN", "secret")
	t.Setenv("APP_SERVER_READ_TIMEOUT", "750ms")
	cfg, err := Load(Source{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "secret" || cfg.Server.ReadTimeout != 750*time.Millisecond {
		t.Fatalf("token=%q timeout=%s", cfg.Token, cfg.Server.ReadTimeout)
	}
}

func TestFirstSearchDirWins(t *testing.T) {
	isolate(t)
	first, second := t.TempDir(), t.TempDir()
	write(t, first, FileName, "log:\n  level: debug\n")
	write(t, second, FileName, "log:\n  level: error\n")
	cfg, err := Load(Source{Dirs: []string{t.TempDir(), first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("level=%q; files are not merged, the first one wins", cfg.Log.Level)
	}
}

func TestSiblingFormatsAreIgnored(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	write(t, dir, "app.json", `{"server": {"port": 1}}`)
	write(t, dir, "app.toml", "[server]\nport = 2\n")
	write(t, dir, FileName, "server:\n  port: 9000\n")
	cfg, err := Load(Source{Dirs: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9000 {
		t.Fatalf("port=%d; the loader read a sibling file", cfg.Server.Port)
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		src  Source
		env  map[string]string
		want string
	}{
		{"explicit file missing", Source{File: filepath.Join(dir, "nope.yaml")}, nil, "no such file"},
		{"malformed file", Source{File: write(t, dir, "bad.yaml", "server: [")}, nil, "read "},
		{"unknown key", Source{File: write(t, dir, "extra.yaml", "server:\n  prot: 1\n")}, nil, "prot"},
		{"bad int from env", Source{}, map[string]string{"APP_SERVER_PORT": "eighty"}, "decode config"},
		{"port out of range", Source{}, map[string]string{"APP_SERVER_PORT": "70000"}, "outside 1-65535"},
		{"zero timeout", Source{}, map[string]string{"APP_SERVER_READ_TIMEOUT": "0s"}, "must be positive"},
		{"unknown level", Source{}, map[string]string{"APP_LOG_LEVEL": "loud"}, "log.level"},
		{"tls without token", Source{}, map[string]string{"APP_SERVER_TLS": "true"}, "needs a token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolate(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			_, err := Load(c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err=%v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestExplicitMissingFileIsNotExist(t *testing.T) {
	isolate(t)
	_, err := Load(Source{File: filepath.Join(t.TempDir(), "nope.yaml")})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
}

func TestOperationalFlagDoesNotBind(t *testing.T) {
	isolate(t)
	if _, err := Load(Source{Flags: flags(t, "--json")}); err != nil {
		t.Fatalf("--json reached UnmarshalExact: %v", err)
	}
}

// rewrite replaces the file's contents with one write and no truncation, so
// the watcher never reads a half-written or empty file. Replacing the file by
// rename is not portable: on macOS Viper sees a removal and stops watching.
func rewrite(t *testing.T, path, body string) {
	t.Helper()
	const size = 64
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	padded := body + "#" + strings.Repeat(" ", size-len(body)-1)
	if _, err := f.WriteAt([]byte(padded), 0); err != nil {
		t.Fatal(err)
	}
}

func TestWatchKeepsLastGoodSnapshot(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), FileName)
	rewrite(t, path, "server:\n  port: 9000\n")
	var current atomic.Pointer[Config]
	applied := make(chan int, 16)
	failed := make(chan error, 16)
	err := Watch(Source{File: path},
		func(c Config) { current.Store(&c); applied <- c.Server.Port },
		func(err error) { failed <- err })
	if err != nil {
		t.Fatal(err)
	}

	rewrite(t, path, "server:\n  port: 9001\n")
	for port := 0; port != 9001; {
		select {
		case port = <-applied:
		case err := <-failed:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("no reload after a valid edit")
		}
	}

	rewrite(t, path, "server:\n  port: 70000\n")
	select {
	case <-failed:
	case <-time.After(5 * time.Second):
		t.Fatal("an invalid edit was not reported")
	}
	if got := current.Load().Server.Port; got != 9001 {
		t.Fatalf("port=%d after a bad edit; want the last good value", got)
	}
}

func TestWatchStaysOnTheResolvedFile(t *testing.T) {
	isolate(t)
	first, second := t.TempDir(), t.TempDir()
	path := filepath.Join(second, FileName)
	rewrite(t, path, "server:\n  port: 9000\n")
	applied := make(chan int, 16)
	failed := make(chan error, 16)
	err := Watch(Source{Dirs: []string{first, second}},
		func(c Config) { applied <- c.Server.Port },
		func(err error) { failed <- err })
	if err != nil {
		t.Fatal(err)
	}

	// A file that discovery would now prefer must not take over the reload.
	rewrite(t, filepath.Join(first, FileName), "server:\n  port: 9100\n")
	rewrite(t, path, "server:\n  port: 9001\n")
	for port := 0; port != 9001; {
		select {
		case port = <-applied:
			if port == 9100 {
				t.Fatal("reload read the newly created higher-priority file")
			}
		case err := <-failed:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("no reload after a valid edit")
		}
	}
}

func TestWatchFailsWhenTheFileIsMissing(t *testing.T) {
	isolate(t)
	path := filepath.Join(t.TempDir(), "missing", FileName)
	err := Watch(Source{File: path}, func(Config) {}, func(error) {})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v; want fs.ErrNotExist", err)
	}
}
