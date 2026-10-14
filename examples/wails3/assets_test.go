package main

import (
	"io/fs"
	"strings"
	"testing"
)

// The embedded frontend has an index.html, so a missing or misplaced build
// fails here instead of as a blank window. The behavior tests serve these
// files through the Wails asset server itself.
func TestEmbeddedAssetsHaveIndex(t *testing.T) {
	b, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `<main id="app">`) {
		t.Fatalf("unexpected index.html:\n%s", b)
	}
}
