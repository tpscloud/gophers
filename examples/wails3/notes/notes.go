package notes

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Note is what the frontend receives. Every field has an explicit json tag,
// because the binding generator and the call bridge both follow
// encoding/json.
type Note struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// NoteInput is what the frontend sends. An empty ID creates a note.
type NoteInput struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Changed is the payload of EventNotesChanged. main registers it with
// application.RegisterEvent, so emitting any other type is an error.
type Changed struct {
	Count int `json:"count"`
}

// EventNotesChanged is emitted after every change to the note list.
const EventNotesChanged = "notes:changed"

const (
	maxTitle = 120
	maxBody  = 64 << 10
)

var (
	// ErrInvalid wraps every validation failure.
	ErrInvalid = errors.New("invalid note")
	// ErrNotFound is returned for an unknown note ID.
	ErrNotFound = errors.New("note not found")
)

// validate checks input again in Go. Any script in the webview can call a
// bound method, so the frontend's checks are a convenience, not a guard.
func validate(in NoteInput) (NoteInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	switch {
	case in.Title == "":
		return in, fmt.Errorf("%w: title is required", ErrInvalid)
	case utf8.RuneCountInString(in.Title) > maxTitle:
		return in, fmt.Errorf("%w: title is longer than %d characters", ErrInvalid, maxTitle)
	case len(in.Body) > maxBody:
		return in, fmt.Errorf("%w: body is larger than %d bytes", ErrInvalid, maxBody)
	}
	return in, nil
}
