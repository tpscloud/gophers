package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tpscloud/gophers/examples/viper/config"
)

// brokenDir holds an app.yaml that fails to parse.
func brokenDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("server: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func execute(t *testing.T, dirs []string, serve Serve, args ...string) (string, error) {
	t.Helper()
	for _, name := range []string{"APP_SERVER_PORT", "APP_LOG_LEVEL", "APP_TOKEN", "APP_SERVER_TLS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	var out, errb bytes.Buffer
	root := NewRoot(dirs, serve)
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errb)
	err := root.ExecuteContext(t.Context())
	return out.String(), err
}

func mustNotServe(t *testing.T) Serve {
	return func(context.Context, config.Config) error {
		t.Fatal("serve ran")
		return nil
	}
}

func TestCompletionIgnoresBrokenConfig(t *testing.T) {
	dirs := []string{brokenDir(t)}
	out, err := execute(t, dirs, mustNotServe(t), "__complete", "serve", "--log-level", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "debug\ninfo\nwarn\nerror\n:4\n") {
		t.Fatalf("out=%q", out)
	}
	if _, err := execute(t, dirs, mustNotServe(t), "completion", "zsh"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, dirs, mustNotServe(t), "help", "serve"); err != nil {
		t.Fatal(err)
	}
}

func TestServeReportsBrokenConfig(t *testing.T) {
	_, err := execute(t, []string{brokenDir(t)}, mustNotServe(t), "serve")
	if err == nil || !strings.Contains(err.Error(), "app.yaml") {
		t.Fatalf("err=%v", err)
	}
}

func TestServeGetsResolvedSnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("server:\n  port: 9000\nlog:\n  level: warn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got config.Config
	serve := func(ctx context.Context, cfg config.Config) error {
		if ctx != t.Context() {
			t.Fatal("context not forwarded")
		}
		got = cfg
		return nil
	}
	if _, err := execute(t, []string{dir}, serve, "serve", "--port", "9200"); err != nil {
		t.Fatal(err)
	}
	if got.Server.Port != 9200 || got.Log.Level != "warn" {
		t.Fatalf("got %+v", got)
	}
}

func TestExplicitConfigMustExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := execute(t, nil, mustNotServe(t), "serve", "--config", missing); err == nil {
		t.Fatal("a missing --config file was ignored")
	}
}
