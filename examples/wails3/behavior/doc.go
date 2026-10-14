// Package behavior tests the Wails v3 behavior the skill states. Claims that
// need a webview or a desktop session cannot be tested here; each is listed
// below with the source it was checked against at v3.0.0-beta.29. Paths are
// inside the module github.com/wailsapp/wails/v3.
//
//   - ServiceStartup gets a context that is cancelled before shutdown; a
//     non-nil error aborts startup and App.Run returns it wrapped, after
//     already started services get their shutdown call:
//     pkg/application/services.go (ServiceStartup doc comment) and
//     pkg/application/application.go (Run, startupService).
//   - Registering two services of the same type fails at Run with "bound
//     method ... is already registered": pkg/application/bindings.go (Add).
//   - Bound calls from a webview overlap: each request is handled on its
//     own goroutine: pkg/application/application.go (Run, "go
//     a.handleWebViewRequest") and internal/assetserver/assetserver_webview.go
//     (dispatchWorkers note).
//   - Windows created before Run open when Run starts; later calls open
//     immediately: pkg/application/window_manager.go (addAndRun) and
//     application.go (runOrDeferToAppRun).
//   - RegisterEvent panics on a duplicate name or a known system event name:
//     pkg/application/events.go (RegisterEvent).
//   - App exposes Window, Dialog, Event, Menu, SystemTray, and other
//     managers as fields: pkg/application/application.go (type App).
//   - A bound call's context is cancelled when the frontend cancels the
//     promise, and carries the calling window under application.WindowKey:
//     pkg/application/messageprocessor_call.go (processCallMethod,
//     processCallCancelMethod).
//   - The JavaScript runtime rejects with ReferenceError, TypeError, or
//     RuntimeError by kind and sets err.cause from the JSON error:
//     internal/runtime/desktop/@wailsio/runtime/src/runtime.ts
//     (runtimeCallWithID).
//   - Generated bindings turn a null array or map result into [] or {}:
//     internal/runtime/desktop/@wailsio/runtime/src/create.ts (Array, Map).
//   - A cancelled save or open dialog returns "" (multiple selection: nil)
//     and a nil error: pkg/application/dialogs.go
//     (PromptForSingleSelection), pkg/application/linux_cgo.go
//     (fileDialogCallback closes the channel when cancelled), and
//     pkg/application/dialogs_darwin.go (path stays NULL unless
//     NSModalResponseOK).
//   - Window.EmitEvent sets the sender to the window name:
//     pkg/application/webview_window.go (EmitEvent).
//   - The default build (no production tag) serves assets from
//     FRONTEND_DEVSERVER_URL when it is set; -tags production never does:
//     internal/assetserver/build_dev.go and build_production.go
//     (NewAssetFileServer).
//   - Linux pkg-config modules: gtk4 and webkitgtk-6.0 by default,
//     gtk+-3.0, webkit2gtk-4.1, and gdk-3.0 with -tags gtk3:
//     pkg/application/linux_cgo.go and linux_cgo_gtk3.go (#cgo lines), plus
//     libsoup-3.0 and gio-unix-2.0 in internal/assetserver/webview.
//     Debian and Ubuntu package names: internal/doctor/packagemanager/apt.go.
//   - macOS links UniformTypeIdentifiers itself:
//     pkg/application/dialogs_darwin.go (#cgo LDFLAGS).
//   - Production builds pass -tags production -trimpath -buildvcs=false
//     -ldflags="-w -s" with CGO_ENABLED=1 on Linux; the bindings task runs
//     wails3 generate bindings -f '<BUILD_FLAGS>' -clean=true:
//     internal/commands/build_assets/linux/Taskfile.yml (build:native) and
//     internal/commands/build_assets/Taskfile.tmpl.yml (generate:bindings).
//   - Generator flags and the frontend/bindings default output directory:
//     internal/flags/bindings.go.
//   - The project template registers services in Options.Services, serves
//     assets with AssetFileServerFS, and opens a window with
//     app.Window.NewWithOptions: internal/templates/_common/main.go.tmpl.
//   - wails3 build runs the build task in Taskfile.yml and passes -tags as
//     EXTRA_TAGS; doctor and tool capabilities exist:
//     internal/commands/task_wrapper.go (Build) and cmd/wails3/main.go.
//   - An unknown method ID or name fails with "unknown bound method id" or
//     "name", which is not a CallError, so the runtime rejects with a plain
//     Error: pkg/application/messageprocessor_call.go and
//     pkg/application/transport_http.go (httpError).
//   - Method IDs are an FNV hash of package path, type, and method:
//     pkg/application/bindings.go (getMethods).
//   - The runtime exports setTransport, and binding calls send object 0
//     with call-id, methodID or methodName, and args:
//     internal/runtime/desktop/@wailsio/runtime/src/runtime.ts and calls.ts.
//   - Frontend events carry name, data, and sender; Events.On returns an
//     unsubscribe function: internal/runtime/desktop/@wailsio/runtime/src/events.ts.
//   - Generated promises have cancel(): .../src/cancellable.ts.
//   - The project templates depend on @wailsio/runtime "latest":
//     internal/templates/*/frontend/package.json. The npm package
//     publishes 3.0.0-beta.29, matching the Go module (npm view).
//   - Server mode: default localhost:8080, WAILS_SERVER_HOST and
//     WAILS_SERVER_PORT override, /health, events over a WebSocket, and
//     native-only calls are no-ops or errors:
//     pkg/application/application_server.go.
//   - -tags strictevents warns once per unregistered event name outside
//     production builds: pkg/application/events_strict.go.
//   - v3 has no webkit2_41 tag: no Go file in the module mentions it.
//   - The module's go.mod declares go 1.25.0.
package behavior
