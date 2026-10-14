# Building, Linux libraries, and packaging

Build tags, `go build` without the Wails CLI, the system libraries a Linux
build needs, and the failures each one causes.

## Build tags

A Wails v2 app picks its mode at compile time:

| Tags | Who passes them | Result |
| --- | --- | --- |
| none | plain `go build`, `go vet`, `go test` | compiles; `wails.Run` returns "Wails applications will not build without the correct build tags." (on Windows it shows a message box instead) |
| `dev` | `wails dev`, with `-gcflags "all=-N -l"` | development build that can serve the frontend from a dev server |
| `desktop,production` | `wails build`, with `-ldflags "-w -s"` (`-H windowsgui` added on Windows) | production build with embedded assets |
| `bindings` | `wails generate module` and the binding step of `wails dev` and `wails build` | writes `wailsjs` and exits; needs `wails.json` in the working directory |

Only `dev`, `production`, and `bindings` select the mode in the v2 module;
no file reads `desktop` at v2.16.0, but the CLI passes it, so keep it. Tags
given to the CLI with `-tags` are added to these defaults.

The untagged build is useful: it never compiles the cgo webview code, so
`go vet ./...` and `go test ./...` run on a machine or CI runner with no GTK,
WebKitGTK, or C compiler. Keep testable code in packages that do not need a
window, and test the bound methods with a fake for the runtime.

Without the CLI, the minimum production build builds the frontend first, so
the embedded directory holds a fresh `index.html`, then compiles:

```sh
(cd frontend && npm install && npm run build)
go build -tags desktop,production -ldflags "-w -s" -o build/bin/app .
```

Put both steps in the same Makefile target or script; a target that only runs
`go build` embeds whatever `frontend/dist` held last, or nothing. The CLI does
the same thing: it runs the `frontend:install` and `frontend:build` commands
from `wails.json` in `frontend:dir` (default `frontend`) before compiling, and
the project templates set them to `npm install` and `npm run build`. Without
`wails.json`, use whatever the frontend's `package.json` calls its build.

On macOS 11 and later, the CLI also sets
`CGO_LDFLAGS="-framework UniformTypeIdentifiers -mmacosx-version-min=10.13"`.
A manual `go build` without it fails to link with
`Undefined symbols ... _OBJC_CLASS_$_UTType`.

The v2.16.0 module declares `go 1.25.0` in its `go.mod`, so the project needs
Go 1.25 or newer, whatever older documentation says.

## Linux libraries

On Linux the webview is GTK3 plus WebKitGTK through cgo. Wails v2 does not
bundle a browser engine. The cgo directives name these pkg-config modules:

- `gtk+-3.0`, always;
- `webkit2gtk-4.0` by default;
- `webkit2gtk-4.1` with the `webkit2_41` tag. This is still GTK3; the 4.1
  is a WebKitGTK API version, not GTK4.

Distributions that dropped the 4.0 API, such as Ubuntu 24.04, need
`libwebkit2gtk-4.1-dev` and `-tags webkit2_41` on every `dev` or
`production` build, CLI or manual. Check before building:

```sh
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 && echo ok
wails doctor   # reports missing packages and how to install them
```

Building needs a C compiler, `pkg-config`, and the `-dev` header packages.
Running the binary needs only the shared libraries. When a user reports a
launch failure, run `ldd ./app | grep 'not found'` on their system.

Embedded HTML, CSS, and JavaScript do not make the binary self-contained: it
still links GTK and WebKitGTK dynamically. Do not call it portable or
dependency-free. A separate command-line tool that does not import Wails can
build with `CGO_ENABLED=0` and has none of these requirements.

Do not run the GUI as root to get around device or file permissions. Fix the
permission (a group, a udev rule, a helper with a narrow interface) instead.

## Packaging

- `wails build -platform windows/amd64` (comma separated for several).
  The Linux and macOS desktop code uses cgo, so build each of those on its
  own platform or with a matching cross toolchain.
- `-nsis` generates a Windows NSIS installer.
- `-webview2 download|embed|browser|error` chooses how a Windows install gets
  the WebView2 runtime; the default is `download`.
- `-clean` empties `build/bin` first; `-o` names the output.

## Symptoms

| Symptom | Cause | Fix |
| --- | --- | --- |
| "will not build without the correct build tags" at launch | built without `dev` or `production` | build with the CLI or add `-tags desktop,production` |
| `Package webkit2gtk-4.0 was not found` | 4.0 API not installed | install the 4.1 dev package and add `webkit2_41` |
| `_OBJC_CLASS_$_UTType` link error on macOS | manual build missing the framework flag | set `CGO_LDFLAGS` as above |
| app exits with "no `index.html` could be found" | frontend not built, or the embed path is wrong | build the frontend; check the `//go:embed` path |
| app exits with "An invalid context was passed" | runtime call with a context that did not come from a lifecycle hook | store and use the `OnStartup` context |
| `wailsjs` stale or missing a method | bindings not regenerated, or the method is unexported | regenerate; export the method |
