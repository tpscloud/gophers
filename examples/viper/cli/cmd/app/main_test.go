package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunReportsConfigErrorOnce(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "70000")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	var out, errb bytes.Buffer
	if code := run([]string{"serve"}, &out, &errb); code != 1 {
		t.Fatalf("code=%d", code)
	}
	if out.Len() != 0 || strings.Count(errb.String(), "app:") != 1 || !strings.Contains(errb.String(), "server.port") {
		t.Fatalf("out=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunServes(t *testing.T) {
	for _, name := range []string{"APP_SERVER_PORT", "APP_SERVER_ADDR", "APP_SERVER_TLS", "APP_LOG_LEVEL", "APP_SERVER_READ_TIMEOUT"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	var out, errb bytes.Buffer
	if code := run([]string{"serve", "--port", "9300"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if out.String() != "listening on 127.0.0.1:9300\n" {
		t.Fatalf("out=%q", out.String())
	}
}
