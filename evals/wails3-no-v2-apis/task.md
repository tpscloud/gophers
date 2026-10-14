# Report export progress from a Wails service

The project's go.mod contains:

```
require github.com/wailsapp/wails/v3 v3.0.0-beta.29
```

`main.go` builds the app with `application.New(application.Options{...})`
and registers `application.NewService(&ExportService{})`. A teammate pasted
this method from an older project:

```go
func (s *ExportService) Export(ctx context.Context, rows []Row) error {
	path, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{Title: "Export"})
	if err != nil {
		return err
	}
	for i := range rows {
		runtime.EventsEmit(ctx, "export:progress", i+1, len(rows))
	}
	return writeCSV(path, rows)
}
```

It does not compile. Rewrite the service so the frontend gets a typed
progress event and the save dialog still works, and add a Go test that runs
without a display.
