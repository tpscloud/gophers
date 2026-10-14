// These tests do not import Wails. They drive the go command and the pinned
// wails3 CLI as subprocesses, so they run on a plain CI runner with cgo off
// and no GTK or WebKit installed.

package behavior

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goCmd runs the go command in dir with cgo off plus extra environment.
func goCmd(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "CGO_ENABLED=0"), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// wailsVersion is the v3 version this module requires, so the CLI used
// below always matches the library.
func wailsVersion(t *testing.T) string {
	t.Helper()
	out, err := goCmd(t, ".", nil, "list", "-m", "-f", "{{.Version}}", "github.com/wailsapp/wails/v3")
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	return strings.TrimSpace(out)
}

// generate runs the pinned wails3 CLI's binding generator in dir and returns
// the output directory. -tags server lets the generator type-check
// pkg/application with cgo off; the output does not depend on the tag.
func generate(t *testing.T, dir string) string {
	t.Helper()
	outDir := t.TempDir()
	out, err := goCmd(t, dir, nil, "run", "github.com/wailsapp/wails/v3/cmd/wails3@"+wailsVersion(t),
		"generate", "bindings", "-ts", "-silent", "-d", outDir, "-f", "-tags server")
	if err != nil {
		t.Fatalf("wails3 generate bindings: %v\n%s", err, out)
	}
	return outDir
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The server-mode suite in ./server runs a real v3 app over HTTP. It needs
// -tags server, which go test ./... does not pass, so run it from here.
func TestServerModeSuite(t *testing.T) {
	if out, err := goCmd(t, ".", nil, "test", "-tags", "server", "-count=1", "./server/"); err != nil {
		t.Fatalf("go test -tags server ./server/: %v\n%s", err, out)
	}
}

// SKILL.md and references/building.md: pkg/application needs cgo on Linux
// and macOS, compiles without it on Windows, and compiles anywhere with
// -tags server.
// Source: pkg/application/linux_cgo.go (//go:build linux && cgo ...) and
// application_server.go (//go:build server).
func TestApplicationNeedsCgoOnLinuxAndMacOS(t *testing.T) {
	build := func(goos string, tags ...string) error {
		args := []string{"build", "-o", os.DevNull}
		if len(tags) > 0 {
			args = append(args, "-tags", strings.Join(tags, ","))
		}
		_, err := goCmd(t, ".", []string{"GOOS=" + goos, "GOARCH=amd64"}, append(args, "./testdata/probe")...)
		return err
	}
	for _, goos := range []string{"linux", "darwin"} {
		if build(goos) == nil {
			t.Errorf("%s without cgo: built, want a compile error", goos)
		}
		if err := build(goos, "server"); err != nil {
			t.Errorf("%s -tags server: %v", goos, err)
		}
	}
	if err := build("windows"); err != nil {
		t.Errorf("windows without cgo: %v", err)
	}
}

// SKILL.md and references/frontend.md: the generator writes one module per
// service under the Go import path, calls methods by numeric ID, drops a
// leading context.Context, and writes typed events for RegisterEvent.
func TestExampleBindings(t *testing.T) {
	out := generate(t, "..")
	pkg := filepath.Join(out, "github.com", "tpscloud", "gophers", "examples", "wails3", "notes")
	svc := read(t, filepath.Join(pkg, "service.ts"))
	want := []string{
		"export function DeleteNote(id: string): $CancellablePromise<void>",
		"export function ExportNotes(): $CancellablePromise<string>",
		"export function ListNotes(): $CancellablePromise<$models.Note[]>",
		"export function SaveNote($in: $models.NoteInput): $CancellablePromise<$models.Note>",
		"$Call.ByID(",
	}
	for _, w := range want {
		if !strings.Contains(svc, w) {
			t.Errorf("service.ts missing %q:\n%s", w, svc)
		}
	}
	if n := strings.Count(svc, "export function"); n != 4 {
		t.Errorf("got %d bound functions, want 4:\n%s", n, svc)
	}
	models := read(t, filepath.Join(pkg, "models.ts"))
	for _, w := range []string{"export class Note {", "export class NoteInput {", "export class Changed {", `"title": string;`} {
		if !strings.Contains(models, w) {
			t.Errorf("models.ts missing %q:\n%s", w, models)
		}
	}
	events := read(t, filepath.Join(out, "github.com", "wailsapp", "wails", "v3", "internal", "eventdata.d.ts"))
	if !strings.Contains(events, `"notes:changed": notes$0.Changed;`) {
		t.Errorf("typed event missing:\n%s", events)
	}
}

// SKILL.md: unlike v2, an exported method named Shutdown is an ordinary
// binding; only ServiceName, ServiceStartup, ServiceShutdown, and ServeHTTP
// are skipped. Models follow encoding/json, a pointer result may be null,
// and an error-only method resolves to void.
// Source: pkg/application/bindings.go (internalServiceMethods).
func TestProbeBindings(t *testing.T) {
	out := generate(t, "testdata/probe")
	dir := filepath.Join(out, "github.com", "tpscloud", "gophers", "examples", "wails3", "behavior", "testdata", "probe")
	ts := read(t, filepath.Join(dir, "probe.ts"))
	for _, w := range []string{
		"export function Shutdown(): $CancellablePromise<void>",
		"export function Check(): $CancellablePromise<void>",
		"export function Find(id: string): $CancellablePromise<$models.Report | null>",
		"export function Rename(name: string): $CancellablePromise<string>",
	} {
		if !strings.Contains(ts, w) {
			t.Errorf("probe.ts missing %q:\n%s", w, ts)
		}
	}
	for _, absent := range []string{"ServiceStartup", "helper"} {
		if strings.Contains(ts, "function "+absent) {
			t.Errorf("%s should not be bound:\n%s", absent, ts)
		}
	}
	models := read(t, filepath.Join(dir, "models.ts"))
	if !strings.Contains(models, `"name": string;`) || !strings.Contains(models, `"Owner": string;`) {
		t.Errorf("tagged or untagged field missing:\n%s", models)
	}
	if strings.Contains(models, "Internal") {
		t.Errorf(`json:"-" field should be left out:\n%s`, models)
	}
}
