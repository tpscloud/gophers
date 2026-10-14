// Package app is the bound Wails application for a small notes editor. The
// exported methods on *App are the frontend contract; everything else stays
// unexported so Wails does not expose it.
package app

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Note is returned to the frontend. Every field has a json tag: Wails
// generates TypeScript only for tagged fields.
type Note struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// NoteInput is what the frontend sends to SaveNote. An empty ID creates a note.
type NoteInput struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// MaxBodyBytes bounds a note body. The frontend may check it too, but Go
// enforces it because any script in the webview can call a bound method.
const MaxBodyBytes = 64 << 10

// ErrInvalidNote marks input the frontend should show as a validation error.
var ErrInvalidNote = errors.New("invalid note")

// ErrNotFound reports an ID that does not exist.
var ErrNotFound = errors.New("note not found")

func validate(in NoteInput) (NoteInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	switch {
	case in.Title == "":
		return in, fmt.Errorf("%w: title is required", ErrInvalidNote)
	case utf8.RuneCountInString(in.Title) > 120:
		return in, fmt.Errorf("%w: title is longer than 120 characters", ErrInvalidNote)
	case len(in.Body) > MaxBodyBytes:
		return in, fmt.Errorf("%w: body is larger than %d bytes", ErrInvalidNote, MaxBodyBytes)
	}
	return in, nil
}
