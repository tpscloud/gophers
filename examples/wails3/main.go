//go:build cgo || windows || server

// Command notes is a Wails v3 desktop app. On macOS and Linux the
// application package needs cgo (and GTK4 with WebKitGTK 6.0 on Linux), so
// this file builds only with cgo, on Windows, or with -tags server.
// main_nocgo.go stands in otherwise, so go vet and go test still run on a
// machine with cgo disabled.
package main

import (
	"log"

	"github.com/tpscloud/gophers/examples/wails3/notes"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	// Typed event: the binding generator emits a TypeScript type for it,
	// and Emit with any other data type is reported and cancelled.
	application.RegisterEvent[notes.Changed](notes.EventNotesChanged)
}

func main() {
	app := application.New(application.Options{
		Name: "Notes",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// The service needs the app for dialogs and events, so register it after
	// New and before Run.
	app.RegisterService(application.NewService(notes.NewService(desktop{app})))

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  "Notes",
		Width:  960,
		Height: 640,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// desktop implements notes.Native with the Wails managers.
type desktop struct{ app *application.App }

func (d desktop) SaveFile(name string) (string, error) {
	// A cancelled dialog returns "" and a nil error.
	return d.app.Dialog.SaveFile().
		SetFilename(name).
		AddFilter("JSON", "*.json").
		PromptForSingleSelection()
}

func (d desktop) Emit(event string, data any) {
	d.app.Event.Emit(event, data)
}
