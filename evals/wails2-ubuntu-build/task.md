# The desktop build fails on Ubuntu 24.04

A Wails v2.16.0 project has no `wails.json`; its Makefile runs
`go build -o build/app ./cmd/desktop`. On an Ubuntu 24.04 machine the
resulting binary exits immediately with "Wails applications will not build
without the correct build tags." After someone added
`-tags desktop,production`, the build stopped with
"Package webkit2gtk-4.0 was not found in the pkg-config search path".

Fix the Makefile target and list what the machine needs installed. The same
repository also has a GitHub Actions job on ubuntu-24.04 that only runs
`go vet ./...` and `go test ./...`; say whether that job needs the same
packages.
