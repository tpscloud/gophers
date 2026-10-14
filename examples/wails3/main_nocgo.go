//go:build !cgo && !windows && !server

package main

import (
	"fmt"
	"os"
)

// Without cgo, Wails v3 does not compile on macOS or Linux. This stub keeps
// the module buildable there so go vet and go test reach the other packages.
func main() {
	fmt.Fprintln(os.Stderr, "notes: build with CGO_ENABLED=1 (macOS, Linux), on Windows, or with -tags server")
	os.Exit(1)
}
