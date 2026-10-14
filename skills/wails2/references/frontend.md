# Frontend contract and tests

How the frontend reaches bound Go methods, how to keep the two sides in step,
and what a browser test can and cannot prove.

## Generated bindings

`wails dev`, `wails build`, and `wails generate module` write a `wailsjs`
directory next to the frontend (the `wailsjsdir` setting in `wails.json`):

- `wailsjs/go/<package>/<Struct>.js` and `.d.ts`: one function per bound
  method. The JavaScript calls `window['go'][package][Struct][Method](...)`
  and returns a promise.
- `wailsjs/go/models.ts`: a class per struct used as an argument or result.
- `wailsjs/runtime/`: the JavaScript runtime (`EventsOn`, `EventsEmit`,
  window and dialog helpers). `EventsOn` returns a function that removes the
  listener.

The folder name is the Go package name, so a struct in package `app` lands
in `wailsjs/go/app/App.js`. Never edit these files; regenerate them after
changing a bound method's name, parameters, results, or a JSON field. Commit
them only if the project already does.

Without the CLI, the generator is the app itself: build it with
`-tags bindings` and run it from the directory that holds `wails.json`. This
needs no cgo or system libraries:

```sh
go build -tags bindings -o /tmp/app-bindings . && /tmp/app-bindings
```

The bindings build reads `wails.json` from the working directory and fails
without it. A project with no `wails.json` either adds a minimal one
(`{"name": "app", "wailsjsdir": "frontend"}`) or keeps a hand-written adapter.

## Hand-written adapters and the browser preview

Some projects skip `wailsjs` and call `window.go?.<package>?.<Struct>`
through a typed adapter module. Keep that pattern if it exists. When a bound
method changes, update the adapter's TypeScript signature, every caller, and
the test doubles in the same change.

If the frontend also runs in a plain browser (a design preview or a demo),
put the preview behind its own adapter chosen once at startup, for example by
checking whether `window.go` exists. A failed native call must reject and
show an error. It must not fall back to `localStorage` or invent a result,
and opening the preview must not touch the native app's files.

## Handling results in the UI

- Every bound call can reject: Go returned an error, or the arguments did
  not decode into the Go parameter types. Catch it and show
  `String(err)`; by default the rejection value is the Go error text.
- Report success only after the promise resolves. Disable a submit control
  while its call is pending instead of queueing duplicates.
- Expect `null` for a nil Go slice, map, or pointer unless the Go side
  returns an empty value.
- Handle a cancelled dialog (empty string or `null`) as "nothing happened".

## Testing the frontend

Unit and browser tests (Vitest, Playwright, and similar) run without Wails.
Stub `window.go` before the code under test loads, with the same package,
struct, and method names the generated bindings use, and have each stub
return a resolved or rejected promise. Cover the rejection path and the
cancelled-dialog path, not only success. The generated runtime wrappers call
`window.runtime`, so stub that too if the code imports `wailsjs/runtime`.

These tests check the frontend's logic against the contract. They do not
exercise WebKitGTK, WebView2, native dialogs, focus, or window lifecycle. Say
so in the handoff, and launch the built app when that behavior changed.
