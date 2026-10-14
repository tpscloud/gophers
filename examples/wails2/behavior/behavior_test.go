// None of these tests needs a webview, GTK, or WebKit, so they run on a
// plain CI runner.

package behavior

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options"
	optassets "github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// generate builds pkg with -tags bindings and cgo off, runs it next to a
// wails.json, and returns the generated wailsjs directory. This is what
// wails generate module does, without the Wails CLI.
func generate(t *testing.T, pkg string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bindings")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-tags", "bindings", "-o", bin, pkg)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags bindings %s: %v\n%s", pkg, err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "wails.json"), []byte(`{"name":"probe","wailsjsdir":"out"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	run := exec.Command(bin)
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("bindings run: %v\n%s", err, out)
	}
	return filepath.Join(dir, "out", "wailsjs")
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// SKILL.md: every exported method of a bound struct pointer is callable from
// the frontend, except a method passed as a lifecycle hook. Unexported
// methods are never bound.
// Source: internal/binding/reflect.go (method loop) and
// internal/app/app_bindings.go (bindingExemptions).
func TestExportedMethodsAreBound(t *testing.T) {
	js := generate(t, "./probe")
	dts := read(t, filepath.Join(js, "go", "main", "Probe.d.ts"))
	for _, want := range []string{"function Shutdown(", "function Report(", "function Check("} {
		if !strings.Contains(dts, want) {
			t.Errorf("%s missing from Probe.d.ts:\n%s", want, dts)
		}
	}
	for _, absent := range []string{"Startup", "helper"} {
		if strings.Contains(dts, absent) {
			t.Errorf("%s should not be bound:\n%s", absent, dts)
		}
	}
}

// SKILL.md: a method that returns only an error resolves to void; a value
// and an error resolve to the value. Generated model fields follow
// encoding/json: the tag name when there is one, the Go name when there is
// not, and nothing for json:"-". (The v2 docs say untagged fields are left
// out; at v2.16.0 they are not.)
// Source: internal/binding/generate.go (return types).
func TestGeneratedTypes(t *testing.T) {
	js := generate(t, "./probe")
	dts := read(t, filepath.Join(js, "go", "main", "Probe.d.ts"))
	if !strings.Contains(dts, "Check():Promise<void>") {
		t.Errorf("error-only method should resolve to void:\n%s", dts)
	}
	if !strings.Contains(dts, "Report():Promise<main.Report>") {
		t.Errorf("struct result type:\n%s", dts)
	}
	models := read(t, filepath.Join(js, "go", "models.ts"))
	if !strings.Contains(models, `this.name = source["name"]`) {
		t.Errorf("tagged field missing:\n%s", models)
	}
	if !strings.Contains(models, `this.Owner = source["Owner"]`) {
		t.Errorf("untagged field should keep its Go name:\n%s", models)
	}
	if strings.Contains(models, "Internal") {
		t.Errorf(`json:"-" field should be left out:\n%s`, models)
	}
}

// The example app binds exactly its four public operations. startup stays
// unexported and NewOptions is a function, so neither reaches the frontend.
func TestExampleBindings(t *testing.T) {
	js := generate(t, "..")
	dts := read(t, filepath.Join(js, "go", "app", "App.d.ts"))
	want := []string{
		"export function DeleteNote(arg1:string):Promise<void>;",
		"export function ExportNotes():Promise<string>;",
		"export function ListNotes():Promise<Array<app.Note>>;",
		"export function SaveNote(arg1:app.NoteInput):Promise<app.Note>;",
	}
	for _, w := range want {
		if !strings.Contains(dts, w) {
			t.Errorf("missing %q in:\n%s", w, dts)
		}
	}
	if n := strings.Count(dts, "export function"); n != len(want) {
		t.Errorf("got %d bound functions, want %d:\n%s", n, len(want), dts)
	}
	// references/frontend.md: the generated JavaScript looks the method up on
	// window.go at call time, which is what lets a frontend test stub it.
	js2 := read(t, filepath.Join(js, "go", "app", "App.js"))
	if !strings.Contains(js2, "return window['go']['app']['App']['SaveNote'](arg1);") {
		t.Errorf("unexpected App.js:\n%s", js2)
	}
}

// SKILL.md: without the dev, production, or bindings tag, a v2 app compiles
// but wails.Run returns an error. This is why go vet and go test work on a
// machine without GTK or WebKit.
// Source: internal/app/app_default_unix.go.
func TestRunWithoutBuildTags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on Windows the untagged build shows a message box instead")
	}
	err := wails.Run(&options.App{})
	if err == nil || !strings.Contains(err.Error(), "will not build without the correct build tags") {
		t.Fatalf("err = %v", err)
	}
}

// SKILL.md: runtime functions need the context passed to OnStartup. Any
// other context makes the process exit through log.Fatalf; nothing is
// returned to recover from.
// Source: pkg/runtime/runtime.go (getEvents).
func TestRuntimeExitsWithoutLifecycleContext(t *testing.T) {
	if os.Getenv("BEHAVIOR_CHILD") == "1" {
		wailsruntime.EventsEmit(context.Background(), "notes:changed")
		return // not reached
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRuntimeExitsWithoutLifecycleContext$")
	cmd.Env = append(os.Environ(), "BEHAVIOR_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child exited 0:\n%s", out)
	}
	if !strings.Contains(string(out), "An invalid context was passed") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

type quietLog struct{}

func (quietLog) Debug(string, ...any) {}
func (quietLog) Error(string, ...any) {}

// SKILL.md: the asset server finds index.html below the embed root, so
// //go:embed all:frontend/dist needs no fs.Sub. An asset tree without
// index.html, such as an unbuilt frontend, is an error.
// Source: pkg/assetserver/assethandler.go (NewAssetHandler) and fs.go.
func TestAssetsNeedIndexHTML(t *testing.T) {
	built := fstest.MapFS{
		"frontend/dist/index.html": {Data: []byte("<main>ok</main>")},
		"frontend/dist/main.js":    {Data: []byte("1")},
	}
	h, err := assetserver.NewAssetHandler(optassets.Options{Assets: built}, quietLog{})
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"/": "<main>ok</main>", "/main.js": "1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body, _ := io.ReadAll(rec.Result().Body)
		if rec.Code != http.StatusOK || string(body) != want {
			t.Errorf("GET %s: %d %q", path, rec.Code, body)
		}
	}

	unbuilt := fstest.MapFS{"frontend/src/main.ts": {Data: []byte("")}}
	_, err = assetserver.NewAssetHandler(optassets.Options{Assets: unbuilt}, quietLog{})
	if err == nil || !strings.Contains(err.Error(), "no `index.html` could be found") {
		t.Fatalf("err = %v", err)
	}
}
