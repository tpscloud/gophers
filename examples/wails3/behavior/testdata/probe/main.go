// Command probe registers a service with the method and field shapes that
// surprise people, so the behavior tests can read what the binding
// generator writes for each one. It lives under testdata so go vet ./...
// and go test ./... skip it: on macOS and Linux it needs cgo or -tags server.
package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Report has a tagged, an untagged, and an excluded field.
type Report struct {
	Name     string `json:"name"`
	Owner    string
	Internal string `json:"-"`
}

// Probe is registered as a service.
type Probe struct{}

// ServiceStartup is a lifecycle method, so it is not bound.
func (p *Probe) ServiceStartup(context.Context, application.ServiceOptions) error { return nil }

// Shutdown is an ordinary exported method, so the frontend can call it.
func (p *Probe) Shutdown() {}

// Report returns a struct, so the generator writes a model class for it.
func (p *Probe) Report() Report { return Report{} }

// Find returns a pointer, which may be nil.
func (p *Probe) Find(id string) *Report { return nil }

// Check returns only an error.
func (p *Probe) Check() error { return nil }

// Rename takes the call context first; Wails fills it in.
func (p *Probe) Rename(ctx context.Context, name string) (string, error) { return name, ctx.Err() }

// helper is unexported and never bound.
func (p *Probe) helper() {}

func main() {
	p := &Probe{}
	p.helper()
	app := application.New(application.Options{
		Services: []application.Service{application.NewService(p)},
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
