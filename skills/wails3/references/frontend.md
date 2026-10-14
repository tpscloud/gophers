# Bindings, the JavaScript runtime, and frontend tests

How the frontend reaches services, what `wails3 generate bindings` writes,
how errors and events look in JavaScript, and what a browser test can and
cannot prove.

## Generated bindings

`wails3 generate bindings` reads the Go packages, finds every
`application.NewService` call, and writes one directory per Go import path
below `frontend/bindings` (change it with `-d`). For a `Service` type in
package `example.com/app/notes`:

- `frontend/bindings/example.com/app/notes/service.ts`: one function per
  exported method, returning a `CancellablePromise`. A leading
  `context.Context` parameter is left out, an error-only method resolves to
  `void`, and a pointer result is `T | null`.
- `models.ts`: a class per struct used as an argument or result (`-ts -i` writes
  interfaces instead), with fields named as `encoding/json` names them.
- `index.ts`: re-exports the services and models.
- `github.com/wailsapp/wails/v3/internal/eventdata.d.ts` and
  `eventcreate.ts`: types and decoders for events registered with
  `application.RegisterEvent`.

Useful flags: `-ts` for TypeScript, `-clean` (on by default) to empty the
output directory first, `-names` to call methods by name instead of ID, and
`-f '<go build flags>'` to load the packages with the build's tags. The
generator loads packages the way `go build` would, so a `NewService` call in
a file the current tags or cgo setting exclude is missed and the run reports
`0 Services`.

The generated code calls `Call.ByID(<id>, ...)`, where the ID is a hash of
the package path, type name, and method name. Renaming any of them, or moving
the service to another package, changes the ID, so regenerate after every
change to a service's methods, parameters, results, or JSON fields. Never
edit generated files. Commit them only if the project already does.

Generated code imports `@wailsio/runtime`. The project templates depend on
`"latest"`; pin the npm package to the same version as the Go module (for
example `3.0.0-beta.28`) so the runtime and the backend agree.

## Calling and handling results

```ts
import { Service } from "./bindings/example.com/app/notes";

try {
  const note = await Service.SaveNote({ id: "", title, body });
} catch (err) {
  show(err instanceof Error ? err.message : String(err));
}
```

- A Go error rejects with a `RuntimeError` whose `message` is the Go error
  text. Wrong argument types reject with a `TypeError`. When the Go error
  marshals to JSON, `err.cause` holds that JSON. Other failures, such as an
  unknown method ID from stale bindings, reject with a plain `Error`.
- Report success only after the promise resolves, and disable a submit
  control while its call is pending.
- The promise has a `cancel()` method; it cancels the Go method's
  `context.Context`.
- A nil Go slice or map is `null` on the wire. Generated bindings turn it
  into `[]` or `{}`; a direct `Call.ByName` caller sees `null`.
- Handle a cancelled dialog (`""`) as "nothing happened".

## Events

```ts
import { Events } from "@wailsio/runtime";

const off = Events.On("notes:changed", (event) => render(event.data.count));
// later
off();
```

The callback receives a `WailsEvent` with `name`, `data`, and `sender` (the
window name, or empty when Go emitted it). `Events.On` returns a function
that removes the listener; call it when the component goes away.
`Events.Emit(name, data)` sends an event to Go, where a registered name is
decoded into its Go type.

## Without a bundler

The app serves the runtime as an ES module at `/wails/runtime.js`, so a page
with no build step can use `await import("/wails/runtime.js")` and call
`Call.ByName("<import path>.<Type>.<Method>", ...args)`. Prefer generated
bindings in a real frontend, because they type check.

## Testing the frontend

Unit and browser tests (Vitest, Playwright, and similar) run without Wails.
Either mock the generated service modules, or install a fake transport
before the code under test runs: `@wailsio/runtime` exports `setTransport`,
and every runtime call then goes to its `call(objectID, method, windowName,
args)` instead of `fetch`. Binding calls use object ID 0 and carry
`methodID` (or `methodName`) and `args`. Return resolved values and thrown
errors from the fake, and cover the rejection and cancelled-dialog paths, not
only success.

To exercise the real Go side without a desktop, build the app with
`-tags server` and point the browser test at it; see
[building.md](building.md#server-mode).

These tests check the frontend's logic against the contract. They do not
exercise WebKitGTK, WebKit, WebView2, native dialogs, focus, or window
lifecycle. Say so in the handoff, and launch the built app when that
behavior changed.
