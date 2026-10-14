# Building, Linux libraries, and server mode

Build tags, cgo, the `wails3` CLI and its Taskfile, plain `go build`, the
system libraries a Linux build needs, server mode, and the failures each one
causes. Checked against v3.0.0-beta.29, whose `go.mod` declares `go 1.25.0`,
so the project needs Go 1.25 or newer. Taskfiles are generated into each
project, so read the project's own copy before changing a build.

## Build tags

| Tags | Who passes them | Result |
| --- | --- | --- |
| none | plain `go build`, `go vet`, `go test`, dev Taskfile builds | debug build; serves assets from `FRONTEND_DEVSERVER_URL` when that variable is set |
| `production` | `wails3 build` and `wails3 package` through the Taskfile | production build; always serves the embedded assets |
| `gtk3` | the user, for older Linux systems | GTK3 with WebKitGTK 4.1 instead of GTK4 with WebKitGTK 6.0 |
| `server` | `task build:server`, or the user | HTTP server with no window and no cgo; see [Server mode](#server-mode) |
| `strictevents` | the user, for non-production builds | warns once for each event name emitted without `RegisterEvent` |

Unlike v2, there is no tag check: an untagged build is a working debug
build. Pass extra tags to the CLI with `wails3 build -tags gtk3`; the
Taskfile adds them to its own.

## cgo

On Linux and macOS, `pkg/application` needs cgo. With `CGO_ENABLED=0` it
does not compile (`undefined: pointer` on Linux, `undefined: fatalHandler`
on macOS). On Windows it compiles without cgo. `-tags server` compiles
everywhere without cgo.

So a CI job with only Go installed cannot vet or test a package that imports
`pkg/application` for Linux. Keep services in packages that do not import
Wails, inject dialogs and events through an interface, and either test the
Wails wiring with `-tags server` or put the entry point behind a build
constraint with a stub for the cgo-off case:

```go
// examples/wails3/main_nocgo.go (trimmed: the stub main body omitted)
//go:build !cgo && !windows && !server

package main
```

The real `main.go` then carries `//go:build cgo || windows || server`.

## The wails3 CLI and Taskfile

`wails3 build` runs the `build` task in the project's `Taskfile.yml`, which
dispatches to `build/<os>/Taskfile.yml`. On Linux that task runs
`go mod tidy`, builds the frontend (which first runs
`wails3 generate bindings`), generates icons, and then runs:

```sh
CGO_ENABLED=1 go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" -o bin/<app>
```

Without a C compiler, or when cross-compiling, the Linux task builds in a
Docker image instead. `wails3 dev -config ./build/config.yml` runs the dev
loop, `wails3 package` packages a production build, and `wails3 task <name>`
runs any task. `wails3 doctor` reports missing system packages, and
`wails3 tool capabilities` reports whether GTK4 or GTK3 is available.

Use the CLI that matches the module. The binding generator in another beta
can write code that does not match the runtime. Without a global install,
run the pinned one:

```sh
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.29 generate bindings -ts
```

Without the CLI, generate bindings, build the frontend, then run the `go
build` line above yourself.

## Linux libraries

The cgo directives name these pkg-config modules:

- default: `gtk4`, `webkitgtk-6.0`, `libsoup-3.0`, `gio-unix-2.0`, and
  `x11` for global shortcuts;
- with `-tags gtk3`: `gtk+-3.0`, `webkit2gtk-4.1`, `gdk-3.0`,
  `libsoup-3.0`, and `gio-unix-2.0`.

On Debian and Ubuntu the doctor checks for `libgtk-4-dev`,
`libwebkitgtk-6.0-dev`, `build-essential`, and `pkg-config`, and treats
`libgtk-3-dev` with `libwebkit2gtk-4.1-dev` as the legacy pair. Check before
building:

```sh
pkg-config --exists gtk4 webkitgtk-6.0 && echo ok
```

Do not add v2's `webkit2_41` tag; v3 does not read it. The binary links
these libraries dynamically, so embedded assets do not make it portable.
Running it needs the shared libraries, not the `-dev` packages.

## macOS and Windows

On macOS the package links its frameworks itself, including
`UniformTypeIdentifiers`, so a manual `go build` needs no `CGO_LDFLAGS`
(v2 did). Windows builds do not need cgo.

## Server mode

`-tags server` builds the same app as an HTTP server: no window, no cgo, and
the frontend in a browser. `Options.Server` sets the address (default
`localhost:8080`), and `WAILS_SERVER_HOST` and `WAILS_SERVER_PORT` override
it. It serves `/health`, the assets, and `/wails/runtime`, and pushes events
over a WebSocket. Native-only calls (screens, hide and show, the about box)
do nothing or return an error. It is
also a way to test services, bindings, and events end to end on a machine
without GTK.

## Symptoms

| Symptom | Cause | Fix |
| --- | --- | --- |
| `undefined: pointer` or `undefined: fatalHandler` in `pkg/application` | cgo is off on Linux or macOS | build with `CGO_ENABLED=1` and a C compiler, or use `-tags server` for headless tests |
| `Package gtk4 was not found` or `webkitgtk-6.0` not found | GTK4 dev packages missing | install them, or install the GTK3 pair and build with `-tags gtk3` |
| blank window; requests fail with "no `index.html` could be found" | frontend not built, or the embed path is wrong | build the frontend; check the `//go:embed` path |
| generator reports `0 Services` | it loaded the package with tags or cgo that excluded the `NewService` call | pass the build's tags with `-f`, as the Taskfile does |
| frontend call fails with "unknown bound method id" | bindings are stale; IDs hash the package path, type, and method name | regenerate the bindings |
| `does not match registered data type` in `ErrorHandler` | `Emit` got a different type than `RegisterEvent` declared, often a pointer | emit the registered type |
| panic: event "is already registered" | `RegisterEvent` called twice for a name | register each event once, in `init` |
