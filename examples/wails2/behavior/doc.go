// Package behavior tests the Wails v2 behavior the skill states. Claims that
// need a running webview cannot be tested here; each is listed below with the
// source it was checked against at v2.16.0 (paths are inside the module
// github.com/wailsapp/wails/v2 unless they start with website/).
//
//   - Bound calls run on their own goroutine on Linux and macOS:
//     internal/frontend/desktop/linux/frontend.go and
//     internal/frontend/desktop/darwin/frontend.go, processMessage
//     ("go func() { ... f.dispatcher.ProcessMessage ... }").
//   - A returned error rejects the promise with err.Error() unless
//     options.App.ErrorFormatter is set; arguments that do not decode also
//     reject: internal/frontend/dispatcher/calls.go, processCallMessage.
//   - Arguments are decoded with encoding/json into the parameter types:
//     internal/binding/boundMethod.go, ParseArgs.
//   - Lifecycle hooks are exempt from binding only when the hook is the
//     method value itself: internal/app/app_production.go and app_dev.go
//     (bindingExemptions), tested here through the bindings build.
//   - OnBeforeClose returns prevent bool; OnShutdown runs after the main
//     loop ends: pkg/options/options.go and internal/app/app_production.go
//     (Run).
//   - runtime.EventsOn and EventsOnce return a function that cancels the
//     listener: pkg/runtime/events.go.
//   - File dialogs return "" (multiple files: nil) when cancelled:
//     website/docs/reference/runtime/dialog.mdx.
//   - Runtime calls need the context from OnStartup or OnDomReady, and
//     OnDomReady is the hook for work that needs the loaded page:
//     website/docs/reference/runtime/intro.mdx.
//   - A missing index.html stops the app with log.Fatal at startup:
//     internal/frontend/desktop/linux/frontend.go (NewAssetServerMainPage).
//   - wails build passes -tags desktop,production and -ldflags "-w -s";
//     wails dev passes -tags dev: website/docs/guides/manual-builds.mdx.
//     No file in the module has a desktop build constraint.
//   - The Linux webview links gtk+-3.0 and webkit2gtk-4.0, or
//     webkit2gtk-4.1 with the webkit2_41 tag:
//     internal/frontend/desktop/linux/frontend.go (#cgo pkg-config lines).
//     Ubuntu 24.04 needs libwebkit2gtk-4.1-dev with that tag:
//     website/docs/gettingstarted/installation.mdx.
//   - On macOS 11 and later the CLI adds -framework UniformTypeIdentifiers
//     to CGO_LDFLAGS: pkg/commands/build/base.go. A manual go build on macOS
//     without it failed to link (_OBJC_CLASS_$_UTType) when checked.
//   - Before compiling, the CLI runs frontend:install and frontend:build from
//     wails.json in frontend:dir, default "frontend": pkg/commands/build/base.go
//     (BuildFrontend) and internal/project/project.go. Every project template
//     sets them to "npm install" and "npm run build":
//     pkg/templates/templates/*/wails.tmpl.json.
//   - The module's go.mod declares go 1.25.0.
//   - The CLI flags -platform, -nsis, -webview2, -clean, and -o:
//     website/docs/reference/cli.mdx.
//   - Wails v3 (checked at v3.0.0-beta.29) uses application.New and
//     Options.Services (v3/pkg/application), the wails3 CLI (v3/cmd/wails3),
//     and GTK4 with webkitgtk-6.0 on Linux (v3/pkg/application/linux_cgo.go).
package behavior
