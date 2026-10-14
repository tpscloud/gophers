package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/assetserver"
	optassets "github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

type quietLog struct{}

func (quietLog) Debug(string, ...any) {}
func (quietLog) Error(string, ...any) {}

// The embedded frontend serves index.html at / through the same handler Wails
// uses, so a missing or misplaced build fails here instead of at launch.
func TestEmbeddedAssetsServeIndex(t *testing.T) {
	h, err := assetserver.NewAssetHandler(optassets.Options{Assets: assets}, quietLog{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if rec.Code != http.StatusOK || !strings.Contains(string(body), `<main id="app">`) {
		t.Fatalf("GET /: %d\n%s", rec.Code, body)
	}
}
