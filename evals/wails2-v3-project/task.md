# Add a backend method to a Wails app

The project's go.mod contains:

```
require github.com/wailsapp/wails/v3 v3.0.0-beta.28
```

and `main.go` calls `application.New(application.Options{...})`. Add a Go
method the frontend can call to return the app's version string, and show
where to register it.
