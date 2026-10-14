//go:build server

// These tests run a real Wails v3 application in server mode (-tags server):
// the same application, binding, event, and asset code a desktop build uses,
// served over HTTP instead of a webview, so they need no cgo, GTK, or WebKit.
// The untagged tests in package behavior run them with
// go test -tags server, which is how scripts/check.sh reaches them.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Item crosses the bridge as JSON.
type Item struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// FieldError marshals to JSON, so it reaches the frontend as the error's cause.
type FieldError struct {
	Field string `json:"field"`
}

func (e *FieldError) Error() string { return "invalid " + e.Field }

var (
	lifecycle     []string
	lifecycleMu   sync.Mutex
	startCtx      context.Context
	ctxDoneAtStop atomic.Bool
)

func record(s string) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	lifecycle = append(lifecycle, s)
}

// Probe is a service with the method shapes that matter.
type Probe struct{}

func (p *Probe) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	startCtx = ctx
	record("probe start")
	return nil
}

func (p *Probe) ServiceShutdown() error {
	ctxDoneAtStop.Store(startCtx.Err() != nil)
	record("probe stop")
	return nil
}

func (p *Probe) Echo(in Item) (Item, error) { in.Count++; return in, nil }
func (p *Probe) Fail() error                { return errors.New("boom") }
func (p *Probe) FailField() error           { return &FieldError{Field: "title"} }
func (p *Probe) NilSlice() []string         { return nil }
func (p *Probe) Panics() string             { panic("kaboom") }
func (p *Probe) Two() (int, string)         { return 1, "two" }
func (p *Probe) HasContext(ctx context.Context) bool {
	return ctx != nil && ctx.Err() == nil
}
func (p *Probe) helper() {}

// Second only records lifecycle order.
type Second struct{}

func (s *Second) ServiceStartup(context.Context, application.ServiceOptions) error {
	record("second start")
	return nil
}

func (s *Second) ServiceShutdown() error {
	record("second stop")
	return nil
}

var (
	app      *application.App
	baseURL  string
	errsMu   sync.Mutex
	appErrs  []error
	received = make(chan any, 16)
)

const pkg = "github.com/tpscloud/gophers/examples/wails3/behavior/server"

func TestMain(m *testing.M) {
	application.RegisterEvent[Item]("probe:item")

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	baseURL = "http://127.0.0.1:" + strconv.Itoa(port)

	app = application.New(application.Options{
		Name:     "probe",
		Services: []application.Service{application.NewService(&Probe{}), application.NewService(&Second{})},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(fstest.MapFS{
			"frontend/dist/index.html": {Data: []byte("<main>ok</main>")},
		})},
		Server: application.ServerOptions{Host: "127.0.0.1", Port: port},
		ErrorHandler: func(err error) {
			errsMu.Lock()
			defer errsMu.Unlock()
			appErrs = append(appErrs, err)
		},
		DisableDefaultSignalHandler: true,
		Logger:                      slog.New(slog.DiscardHandler),
	})
	app.Event.On("probe:item", func(e *application.CustomEvent) { received <- e.Data })
	app.Event.On("probe:loose", func(e *application.CustomEvent) { received <- e.Data })

	runErr := make(chan error, 1)
	go func() { runErr <- app.Run() }()
	waitHealthy()

	code := m.Run()

	app.Quit()
	if err := <-runErr; err != nil {
		fmt.Println("Run:", err)
		code = 1
	}
	// Services start in registration order and stop in reverse, after the
	// application context is cancelled.
	want := []string{"probe start", "second start", "second stop", "probe stop"}
	if !slices.Equal(lifecycle, want) || !ctxDoneAtStop.Load() {
		fmt.Printf("lifecycle = %v (ctx done at stop: %v), want %v\n", lifecycle, ctxDoneAtStop.Load(), want)
		code = 1
	}
	os.Exit(code)
}

func waitHealthy() {
	for range 100 {
		if resp, err := http.Get(baseURL + "/health"); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	panic("server mode app did not start")
}

var callID atomic.Int64

// call does what the generated bindings do: POST /wails/runtime with object
// 0 (Call), method 0 (by name or ID), and JSON arguments.
func call(t *testing.T, method string, args ...any) (int, string, map[string]any) {
	t.Helper()
	if args == nil {
		args = []any{}
	}
	body, _ := json.Marshal(map[string]any{
		"object": 0, "method": 0,
		"args": map[string]any{
			"call-id":    strconv.FormatInt(callID.Add(1), 10),
			"methodName": pkg + "." + method,
			"args":       args,
		},
	})
	resp, err := http.Post(baseURL+"/wails/runtime", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(raw, &obj)
	}
	return resp.StatusCode, string(raw), obj
}

// SKILL.md: arguments and results cross as JSON; a struct argument is
// decoded into the Go parameter type.
func TestCallRoundTrip(t *testing.T) {
	code, raw, _ := call(t, "Probe.Echo", Item{Name: "a", Count: 1})
	if code != http.StatusOK || raw != `{"name":"a","count":2}` {
		t.Fatalf("%d %s", code, raw)
	}
	// Several non-error results arrive as a JSON array.
	if code, raw, _ := call(t, "Probe.Two"); code != http.StatusOK || raw != `[1,"two"]` {
		t.Fatalf("%d %s", code, raw)
	}
	// A nil slice is null on the wire. Generated bindings turn it into [],
	// raw Call.ByName callers do not.
	if code, raw, _ := call(t, "Probe.NilSlice"); code != http.StatusOK || raw != "null" {
		t.Fatalf("%d %s", code, raw)
	}
}

// SKILL.md: a returned error rejects the promise with a RuntimeError whose
// message is err.Error(); an error that marshals to JSON travels as cause.
func TestErrorsRejectWithKindAndCause(t *testing.T) {
	code, raw, obj := call(t, "Probe.Fail")
	if code != http.StatusUnprocessableEntity || obj["kind"] != "RuntimeError" || obj["message"] != "boom" {
		t.Fatalf("%d %s", code, raw)
	}
	_, raw, obj = call(t, "Probe.FailField")
	cause, _ := obj["cause"].(map[string]any)
	if obj["message"] != "invalid title" || cause["field"] != "title" {
		t.Fatalf("FailField: %s", raw)
	}
	// Arguments that do not decode reject with a TypeError.
	_, raw, obj = call(t, "Probe.Echo", "not an object")
	if obj["kind"] != "TypeError" {
		t.Fatalf("bad argument: %s", raw)
	}
}

// SKILL.md: a panic in a bound method rejects that call; the app keeps running.
func TestPanicRejectsCall(t *testing.T) {
	_, raw, obj := call(t, "Probe.Panics")
	if obj["kind"] != "RuntimeError" || !strings.Contains(raw, "panic: kaboom") {
		t.Fatalf("%s", raw)
	}
	if code, _, _ := call(t, "Probe.Two"); code != http.StatusOK {
		t.Fatal("app stopped answering after a panic")
	}
}

// SKILL.md: exported methods are bindings, except ServiceName,
// ServiceStartup, ServiceShutdown, and ServeHTTP. Unexported methods never are.
// A leading context.Context parameter is filled in by Wails.
func TestWhatIsBound(t *testing.T) {
	for _, m := range []string{"Probe.ServiceStartup", "Probe.ServiceShutdown", "Probe.helper"} {
		code, raw, _ := call(t, m)
		if code == http.StatusOK || !strings.Contains(raw, "unknown bound method name") {
			t.Errorf("%s: %d %s", m, code, raw)
		}
	}
	if code, raw, _ := call(t, "Probe.HasContext"); code != http.StatusOK || raw != "true" {
		t.Fatalf("HasContext: %d %s", code, raw)
	}
}

// SKILL.md: Emit with the registered type reaches Go listeners; any other
// type, even a pointer to it, is reported to ErrorHandler and cancelled.
func TestTypedEvents(t *testing.T) {
	if cancelled := app.Event.Emit("probe:item", Item{Name: "ok"}); cancelled {
		t.Fatal("registered type was cancelled")
	}
	if got := <-received; got != (Item{Name: "ok"}) {
		t.Fatalf("listener got %#v", got)
	}
	if cancelled := app.Event.Emit("probe:item", &Item{Name: "ptr"}); !cancelled {
		t.Fatal("pointer data was not cancelled")
	}
	errsMu.Lock()
	n := len(appErrs)
	last := fmt.Sprint(appErrs)
	errsMu.Unlock()
	if n == 0 || !strings.Contains(last, "does not match registered data type") {
		t.Fatalf("ErrorHandler got %v", last)
	}
	select {
	case got := <-received:
		t.Fatalf("cancelled event delivered: %#v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// SKILL.md: events the frontend emits arrive in Go decoded into the
// registered type; unregistered names arrive as generic JSON values.
func TestFrontendEventsDecode(t *testing.T) {
	emit := func(name string, data any) {
		body, _ := json.Marshal(map[string]any{"object": 3, "method": 0, "args": map[string]any{"name": name, "data": data}})
		resp, err := http.Post(baseURL+"/wails/runtime", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	emit("probe:item", map[string]any{"name": "js", "count": 3})
	if got := <-received; got != (Item{Name: "js", Count: 3}) {
		t.Fatalf("registered: %#v", got)
	}
	emit("probe:loose", map[string]any{"name": "js"})
	if got, ok := (<-received).(map[string]any); !ok || got["name"] != "js" {
		t.Fatalf("unregistered: %#v", got)
	}
}

// Late is registered after Run has started.
type Late struct{}

func (l *Late) Hello() string { return "hi" }

// SKILL.md: a service registered after Run starts is dropped, and the
// error goes to Options.ErrorHandler.
func TestRegisterAfterRunIsDropped(t *testing.T) {
	app.RegisterService(application.NewService(&Late{}))
	errsMu.Lock()
	last := fmt.Sprint(appErrs)
	errsMu.Unlock()
	if !strings.Contains(last, "services must be registered before running the application") {
		t.Fatalf("ErrorHandler got %v", last)
	}
	if code, raw, _ := call(t, "Late.Hello"); code == http.StatusOK {
		t.Fatalf("late service was bound: %s", raw)
	}
}

// SKILL.md: application.New returns the first App on every later call.
func TestNewIsASingleton(t *testing.T) {
	if again := application.New(application.Options{Name: "other"}); again != app {
		t.Fatal("second New returned a new App")
	}
}

// SKILL.md: AssetFileServerFS finds index.html below the embed root, and
// the app serves the runtime, an ES module, at /wails/runtime.js.
func TestAssets(t *testing.T) {
	for path, want := range map[string]string{"/": "<main>ok</main>", "/wails/runtime.js": " as Call"} {
		resp, err := http.Get(baseURL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), want) || len(b) == 0 {
			t.Errorf("GET %s: %d %.80q", path, resp.StatusCode, b)
		}
	}
}

// SKILL.md: an asset tree without index.html is not a startup error in v3;
// every request fails with a 500 that names the problem.
func TestMissingIndexIsAPerRequestError(t *testing.T) {
	h := application.AssetFileServerFS(fstest.MapFS{"src/main.ts": {Data: []byte("")}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "no `index.html` could be found") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
