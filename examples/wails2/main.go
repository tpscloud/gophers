// Command notes is a Wails v2 desktop app. Build it with the Wails CLI
// (wails build) or with go build -tags desktop,production. Without those
// tags it compiles, vets, and tests, but wails.Run returns an error.
package main

import (
	"embed"
	"log"

	"github.com/tpscloud/gophers/examples/wails2/app"
	"github.com/wailsapp/wails/v2"
)

// The frontend build writes frontend/dist. The all: prefix keeps files whose
// names start with . or _, which bundlers sometimes emit.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := wails.Run(app.NewOptions(app.New(nil), assets)); err != nil {
		log.Fatal(err)
	}
}
