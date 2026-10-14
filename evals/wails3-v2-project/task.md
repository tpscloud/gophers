# Add a backend method to a Wails app

The project's go.mod contains:

```
require github.com/wailsapp/wails/v2 v2.16.0
```

and `main.go` calls `wails.Run(&options.App{...})` with `Bind: []any{app}`.
Add a Go method the frontend can call to return the app's version string,
and show where to register it.
