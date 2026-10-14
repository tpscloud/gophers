package main

import "embed"

// The frontend build writes frontend/dist. The all: prefix keeps files whose
// names start with . or _, which bundlers sometimes emit. The Wails asset
// server finds index.html below the embed root, so no fs.Sub is needed.
//
//go:embed all:frontend/dist
var assets embed.FS
