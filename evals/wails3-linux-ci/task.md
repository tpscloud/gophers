# Wails v3 on Ubuntu: CI and the release build

A Wails v3.0.0-beta.28 project builds fine on a Mac. Its GitHub Actions job
on ubuntu-24.04 installs only Go and runs `go vet ./...` and
`go test ./...`, and it fails inside
`github.com/wailsapp/wails/v3/pkg/application` with errors such as
`undefined: pointer`. The job sets `CGO_ENABLED=0`. Someone suggested adding
the `webkit2_41` tag and installing `libwebkit2gtk-4.1-dev`.

Explain the failure, fix the CI job so it still vets and tests the services,
and list what an Ubuntu machine needs to build the release binary.
