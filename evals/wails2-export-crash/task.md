# Export quits the app

A Wails v2.16.0 desktop app has this bound method:

```go
func (a *App) Export(data string) error {
	path, err := runtime.SaveFileDialog(context.Background(), runtime.SaveDialogOptions{Title: "Export"})
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}
```

Clicking Export closes the whole app, and the terminal shows
"An invalid context was passed". The team also wants a Go test for Export
that runs in CI without a display.

Explain the crash, fix the method, and write the test.
