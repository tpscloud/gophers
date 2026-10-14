---
name: wails3
description: "Build, debug, and package Wails v3 desktop apps (module github.com/wailsapp/wails/v3, a prerelease) that pair a Go backend with a web frontend: application.New and its Options, services registered with application.NewService and their startup and shutdown hooks, bindings from wails3 generate bindings, errors and panics as rejected promises, typed events, dialogs, windows, embedded assets, the wails3 CLI and Taskfile, server mode, and cgo with GTK4 and WebKitGTK 6.0 on Linux. Use when writing or changing a Wails v3 app, adding a service method, or debugging a v3 build, binding, or blank window. Not for Wails v2 (module github.com/wailsapp/wails/v2; use the wails2 skill), Electron or Tauri apps, or Go web servers."
license: "MIT"
---

# Wails v3

Wails v3 runs a Go backend and a web frontend in native windows. You create
the app with `application.New`, register Go structs as services, open windows
through `app.Window`, and reach dialogs and events through managers on the
app (`app.Dialog`, `app.Event`). The exported methods of each service become
promise-returning functions in generated TypeScript or JavaScript. v3 is a
prerelease, its API changes between betas, and it shares almost no API with
v2, so most bugs come from code written for v2 or for a different beta.

## Workflow

1. Read `go.mod`. If it requires `github.com/wailsapp/wails/v2`, stop and use
   the `wails2` skill; without it, read the pinned v2 module source. v2 and
   v3 APIs do not mix, so do not migrate between majors, or move to another
   v3 beta, unless asked. If the project pins a v3 version other than the one
   in [Sources](#sources), read that version's source before trusting a
   detail here: `go doc github.com/wailsapp/wails/v3/pkg/application`, or the
   module cache under `$(go env GOMODCACHE)/github.com/wailsapp/wails/v3@<version>`.
2. Find `application.New(application.Options{...})`, the services, the
   windows, the `//go:embed` asset directory, and how the project builds:
   `Taskfile.yml` with the `wails3` CLI, or plain `go build` from a script.
   Keep the existing workflow.
3. Change the Go side first, following [The service contract](#the-service-contract).
   For generated bindings, the JavaScript runtime, and frontend tests, see
   [references/frontend.md](references/frontend.md).
4. For build tags, cgo, Linux libraries, the Taskfile, and server mode, see
   [references/building.md](references/building.md).
5. Before finishing, run [Verify](#verify).

## Telling v3 from v2

| Concern | Wails v3 | Wails v2 (never in v3 code) |
| --- | --- | --- |
| Entry point | `application.New(application.Options{...})`, then `app.Run()` | `wails.Run(&options.App{...})` |
| Exposing Go | `Services: []application.Service{application.NewService(&T{})}` | `Bind: []any{&T{}}` |
| Lifecycle | `ServiceStartup(ctx, application.ServiceOptions) error` and `ServiceShutdown() error` on the service; `Options.OnShutdown` | `OnStartup`, `OnDomReady`, `OnShutdown`, `OnBeforeClose` in `options.App` |
| Runtime calls | `app.Event.Emit`, `app.Dialog.SaveFile()`, `app.Window.NewWithOptions` | `runtime.EventsEmit(ctx, ...)`, `runtime.SaveFileDialog(ctx, ...)` |
| Frontend | `frontend/bindings/<import path>/` and `@wailsio/runtime` | `wailsjs/go/...` and `window.go` |
| Tooling | `wails3` CLI, `Taskfile.yml`, `build/config.yml` | `wails` CLI, `wails.json` |
| Linux | `gtk4` and `webkitgtk-6.0`; `-tags gtk3` for GTK3 | `gtk+-3.0` and `webkit2gtk-4.0` or `-4.1` |

Imports from `github.com/wailsapp/wails/v2/...`, a `runtime.` call that
takes a context, or a `wailsjs` path in a v3 project is a mixing mistake.

## The service contract

Keep each service thin, its exported methods the whole frontend API, and
everything native behind an interface the tests can replace:

```go
// examples/wails3/notes/service.go (trimmed: storage and the other methods omitted)
type Native interface {
	SaveFile(defaultName string) (string, error) // "" when cancelled
	Emit(event string, data any)
}

type Service struct {
	mu     sync.Mutex // bound calls can run concurrently
	notes  []Note
	native Native
}

func NewService(native Native) *Service { return &Service{native: native} }

func (s *Service) SaveNote(in NoteInput) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err // rejects the frontend promise
	}
	return s.store(Note(in)) // locks s.mu and emits EventNotesChanged
}
```

The app wires it up in `main`:

```go
// examples/wails3/main.go (trimmed: init, window size, Mac options, Emit, and the file filter omitted)
// assets is the //go:embed all:frontend/dist filesystem (see Assets below).
func main() {
	app := application.New(application.Options{
		Name:   "Notes",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	app.RegisterService(application.NewService(notes.NewService(desktop{app})))
	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Notes", URL: "/"})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// desktop implements notes.Native; the service's tests use a fake instead.
type desktop struct{ app *application.App }

func (d desktop) SaveFile(name string) (string, error) {
	return d.app.Dialog.SaveFile().SetFilename(name).PromptForSingleSelection()
}
```

- `application.NewService` takes a pointer to a named, non-generic struct.
  Register one instance per type, in `Options.Services` or with
  `app.RegisterService` before `Run`. A service registered after `Run` starts
  is dropped and the error goes to `Options.ErrorHandler`.
- Every exported method on the pointer type is a binding, except
  `ServiceName`, `ServiceStartup`, `ServiceShutdown`, and `ServeHTTP`.
  v2's exemption for methods passed as lifecycle hooks does not exist, so an
  exported `Startup` or `Shutdown` method is callable from JavaScript. Keep
  helpers unexported and wiring in plain functions.
- Return `T`, `error`, or `(T, error)`; several non-error results arrive as
  an array. A non-nil error rejects the promise with a `RuntimeError` whose
  message is `err.Error()`. When the error value marshals to JSON, that JSON
  becomes the rejection's `cause`; `Options.MarshalError` or
  `ServiceOptions.MarshalError` changes it. Arguments that do not decode
  reject with a `TypeError`. A panic rejects that call and the app keeps
  running.
- Arguments and results cross as JSON. Give every field an explicit `json`
  tag; an untagged field keeps its Go name and `json:"-"` drops it. A nil
  slice is `null` on the wire, so return an empty slice.
- A first parameter of type `context.Context` is filled in by Wails and left
  out of the generated signature. Wails cancels it when the frontend cancels
  the promise, and it carries the calling window under
  `application.WindowKey`.
- Validate input again in Go: any script in the webview can call a binding.
  Calls from a webview run on their own goroutines and overlap, so guard
  shared state with a mutex.
- `ServiceStartup` runs in registration order before windows open; its
  context is cancelled before shutdown, and a returned error makes `Run`
  return it. `ServiceShutdown` runs in reverse order.
- `application.New` returns the first `App` on every later call. Build it
  once in `main`.

## Events, dialogs, and windows

- Register each custom event once, in `init`, by calling
  `application.RegisterEvent` with the payload type `T` as its type argument
  and the event name (see `examples/wails3/main.go`). The generator then writes TypeScript types for them, and `app.Event.Emit`
  checks the data type exactly: a `*T` for a `T` event is reported to
  `ErrorHandler` and cancelled. Events the frontend emits arrive in Go
  decoded into `T`.
- `app.Event.On` returns a function that removes the listener. Use method
  results for request and response, and events for progress or pushed state.
- Dialogs come from `app.Dialog`: `SaveFile()`, `OpenFile()`, and message
  dialogs (`Info`, `Question`, `Warning`, `Error`). A cancelled file dialog
  returns `""` (multiple selection: `nil`) and a nil error; treat that as
  "nothing happened".
- `app.Window.NewWithOptions` opens a window; call it again for more. Windows
  created before `Run` open when it starts. Give each a `Name` and find it
  later with `app.Window.GetByName`. Menus and the system tray hang off
  `app.Menu` and `app.SystemTray`.

## Assets

`//go:embed all:frontend/dist` with
`application.AssetFileServerFS(assets)` serves the built frontend; the asset
server finds `index.html` below the embed root, so `fs.Sub` is not needed.
Build the frontend before `go build`. In v3 a missing `index.html` does not
stop startup: every request fails with a 500 that says "no `index.html` could
be found", which shows as a blank or error window. The app serves its
JavaScript runtime at `/wails/runtime.js`.

## Verify

Before finishing, run `go vet ./...` and `go test ./...` for the changed
packages, and check:

- no v2 import, type, or runtime call appears in the change;
- invalid input to each changed binding returns an error and changes nothing;
- a cancelled dialog writes nothing and returns no error;
- native calls go through an interface faked in tests;
- bindings were regenerated with the project's `wails3` version and build
  tags, not edited, and the frontend type checks against them; the handoff
  names the command (`wails3 generate bindings` with its flags, or the
  Taskfile task) so the user can rerun it after the next service change;
- the frontend build is fresh, so `index.html` is in the embedded assets;
- the native build succeeds with the project's tags when Wails integration
  changed.

When a check fails, match the symptom in
[references/building.md](references/building.md) or the sections above, fix
it, and rerun every check. Server mode and browser tests do not exercise
WebKitGTK, WebKit, WebView2, native dialogs, or window lifecycle; where a
desktop session is available, launch the built app, and state in the handoff
what native behavior was not exercised.

## Sources

Verified against Wails v3.0.0-beta.29, a prerelease. Each stated behavior has a test or a source citation in the
[examples module](https://github.com/tpscloud/gophers/tree/main/examples/wails3).
When the project's version differs, trust its module source over this skill.

- [Application package](https://github.com/wailsapp/wails/tree/v3.0.0-beta.29/v3/pkg/application)
- [Binding generator](https://github.com/wailsapp/wails/tree/v3.0.0-beta.29/v3/internal/generator)
- [JavaScript runtime](https://github.com/wailsapp/wails/tree/v3.0.0-beta.29/v3/internal/runtime/desktop/@wailsio/runtime/src)
- [Project template and Taskfiles](https://github.com/wailsapp/wails/tree/v3.0.0-beta.29/v3/internal/commands/build_assets)
- [API documentation](https://pkg.go.dev/github.com/wailsapp/wails/v3@v3.0.0-beta.29/pkg/application)
