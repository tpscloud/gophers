// Command probe binds a struct with the shapes that surprise people, so the
// behavior tests can read what Wails generates for each one. Build it with
// -tags bindings and run it next to a wails.json to write the bindings.
package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// Report has a tagged, an untagged, and an excluded field.
type Report struct {
	Name     string `json:"name"`
	Owner    string
	Internal string `json:"-"`
}

// Probe is bound to the frontend.
type Probe struct{}

// Startup is exported but passed as OnStartup, so Wails exempts it.
func (p *Probe) Startup(context.Context) {}

// Shutdown is exported and not passed to any hook, so the frontend can call it.
func (p *Probe) Shutdown(context.Context) {}

// Report returns a struct, so Wails generates a model class for it.
func (p *Probe) Report() Report { return Report{} }

// Check returns only an error.
func (p *Probe) Check() error { return nil }

// helper is unexported and never bound.
func (p *Probe) helper() {}

func main() {
	p := &Probe{}
	p.helper()
	if err := wails.Run(&options.App{OnStartup: p.Startup, Bind: []any{p}}); err != nil {
		log.Fatal(err)
	}
}
