---
name: wails2
description: "Build, debug, and package Wails v2 desktop apps that pair a Go backend with a web frontend: bound Go methods and the generated wailsjs bindings, JSON types, errors as rejected promises, the OnStartup context and runtime calls, events and native dialogs, embedded frontend assets, build tags for go build, and Linux GTK3 and WebKitGTK dependencies. Use when writing or changing a Wails v2 app, adding a bound method, or debugging a build that fails on tags, cgo, missing libraries, or a blank window. Not for Wails v3 (module github.com/wailsapp/wails/v3; use the wails3 skill), Electron or Tauri apps, or Go web servers."
license: "MIT"
---

# Wails

Wails v2 runs a Go backend and a web frontend in one native window. Go
methods on a bound struct become promise-returning JavaScript functions, the
runtime package talks to the window through the context Wails hands to
`OnStartup`, and the frontend build is embedded in the binary. Most bugs come
from the edges of that contract: a method exposed by accident, a runtime call
with the wrong context, a build without the right tags, or a missing system
library on Linux.

## Workflow

1. Read `go.mod`. If it requires `github.com/wailsapp/wails/v3`, stop and use
   the `wails3` skill, or read the pinned v3 module source if that skill is not
   installed. v2 and v3 APIs do not mix, so do not migrate between majors
   unless asked. On v2, use only APIs present in the pinned version.
2. Find the entry point (`wails.Run(&options.App{...})`), the bound structs in
   `Bind`, the lifecycle hooks, the embedded asset directory, and how the
   project builds: `wails.json` with the Wails CLI, or plain `go build` from a
   Makefile or script. Keep the existing workflow.
3. Change the Go side first, following [The bound contract](#the-bound-contract).
   For frontend callers, generated bindings, and frontend tests, see
   [references/frontend.md](references/frontend.md).
4. For build tags, `go build` without the CLI, Linux libraries, and packaging,
   see [references/building.md](references/building.md).
5. Before finishing, run [Verify](#verify).

## The bound contract

Keep the bound struct thin, its exported methods the whole frontend API, and
everything native behind an interface the tests can replace:

```go
// examples/wails2/app/app.go (trimmed: storage, the other methods, and helpers omitted)
type Native interface {
	SaveFileDialog(runtime.SaveDialogOptions) (string, error)
	Emit(event string, data ...any)
}

type App struct {
	mu     sync.Mutex // bound calls run concurrently on Linux and macOS
	notes  []Note
	native Native
}

func NewOptions(a *App, assets fs.FS) *options.App {
	return &options.App{
		Title:       "Notes",
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   a.startup, // unexported, so it is not bound
		Bind:        []any{a},  // exported methods become JavaScript functions
	}
}

func (a *App) startup(ctx context.Context) {
	if a.native == nil {
		a.native = wailsNative{ctx: ctx}
	}
}

func (a *App) SaveNote(in NoteInput) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err // rejects the frontend promise
	}
	return a.store(Note(in)) // locks a.mu and emits EventNotesChanged
}

// wailsNative calls the Wails runtime with the context from OnStartup.
type wailsNative struct{ ctx context.Context }

func (n wailsNative) SaveFileDialog(o runtime.SaveDialogOptions) (string, error) {
	return runtime.SaveFileDialog(n.ctx, o)
}
```

```go
// examples/wails2/main.go
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := wails.Run(app.NewOptions(app.New(nil), assets)); err != nil {
		log.Fatal(err)
	}
}
```

- `Bind` takes pointers to structs. Every exported method on the pointer type
  becomes callable from the frontend; unexported methods never are. The one
  exception is a method passed as a lifecycle hook (`OnStartup`, `OnDomReady`,
  `OnShutdown`, `OnBeforeClose`). An exported `Shutdown` that is not wired to
  a hook is callable from JavaScript. Keep hooks unexported and build options
  in a plain function, not a method.
- Return `T`, `error`, or `(T, error)`. A non-nil error rejects the promise
  with `err.Error()` as a string unless `options.App.ErrorFormatter` is set.
  An error-only method resolves to `void`.
- Arguments and results cross as JSON. Give every field an explicit `json`
  tag. Generated TypeScript follows `encoding/json`: an untagged field keeps
  its Go name and `json:"-"` drops it. Return an empty slice, not nil, for
  arrays; the frontend receives nil as `null`.
- Validate input again in Go. Any script loaded in the webview can call a
  bound method, so sizes, IDs, and allowed operations are Go's job. Do not
  bind generic shell execution or unrestricted file access.
- On Linux and macOS each bound call runs on its own goroutine, so shared
  bound state needs a mutex.

## Runtime, lifecycle, and dialogs

- Runtime functions (`runtime.EventsEmit`, `runtime.SaveFileDialog`, window
  calls) need the context passed to `OnStartup` or `OnDomReady`. With
  `context.Background()` or a test context, the runtime calls `log.Fatalf`
  and the process exits. Store the lifecycle context once and wrap the calls
  behind an interface like `Native` above.
- Calls that need the loaded page belong in `OnDomReady`, not `OnStartup`.
  `OnBeforeClose` returns `true` to keep the window open; `OnShutdown` runs
  after the window closes. Cancel app-owned goroutines in `OnShutdown`.
- Use method results for request and response, and events for progress or
  pushed state. `runtime.EventsOn` returns a function that removes the
  listener; call it when the owner goes away.
- File dialogs return `""` (or a nil slice for multiple files) when the user
  cancels. Treat that as a normal outcome: return without writing and without
  an error.

## Assets

`//go:embed all:frontend/dist` embeds the built frontend; `all:` keeps files
whose names start with `.` or `_`. The asset server looks for `index.html`
below the embed root, so `fs.Sub` is not needed. Build the frontend before
`go build`, in the same Makefile target or script that compiles. If `frontend/dist` has no `index.html`, the app exits at startup
with "no `index.html` could be found in your Assets fs.FS". A production
binary must start without a Vite dev server.

## Verify

Before finishing, run `go vet ./...` and `go test ./...` for the changed
packages, and check:

- invalid input to each changed bound method returns an error and changes
  nothing;
- a cancelled dialog writes nothing and returns no error;
- runtime calls go through the stored lifecycle context, faked in tests;
- generated bindings were regenerated, not edited, and the frontend type
  checks against them;
- the frontend build is fresh and `index.html` is served from the embedded
  assets;
- the native build succeeds with the project's tags when Wails integration
  changed.

When a check fails, match the symptom in
[references/building.md](references/building.md) or the sections above, fix
it, and rerun every check. A browser test with a mocked bridge does not test
WebKitGTK, WebView2, native dialogs, or window lifecycle; where a desktop
session is available, launch the built app as a normal user, and state in the
handoff what native behavior was not exercised.

## Sources

Verified against Wails v2.16.0. Each stated behavior has a test
or a source citation in the
[examples module](https://github.com/tpscloud/gophers/tree/main/examples/wails2).
When the project's version differs, trust its module source over this skill.

- [Binding and generated code](https://github.com/wailsapp/wails/tree/v2.16.0/v2/internal/binding)
- [Runtime package](https://github.com/wailsapp/wails/tree/v2.16.0/v2/pkg/runtime)
- [Application options](https://github.com/wailsapp/wails/blob/v2.16.0/v2/pkg/options/options.go)
- [Documentation source](https://github.com/wailsapp/wails/tree/v2.16.0/website/docs); the
  rendered site at wails.io may refuse scripted fetches.
- [API documentation](https://pkg.go.dev/github.com/wailsapp/wails/v2@v2.16.0)
