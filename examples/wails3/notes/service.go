// Package notes is the testable core of the example app. It does not import
// Wails, so its tests run on any machine, with or without cgo, GTK, or
// WebKitGTK. main wraps a *Service with application.NewService and passes
// the Wails dialog and event APIs in through Native.
package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
)

// Native is the part of Wails the service uses. main implements it with
// app.Dialog and app.Event; tests pass a fake.
type Native interface {
	// SaveFile asks for a path and returns "" when the user cancels.
	SaveFile(defaultName string) (string, error)
	Emit(event string, data any)
}

// Service is registered with application.NewService. Every exported method
// on *Service becomes a frontend binding, so helpers stay unexported.
type Service struct {
	mu     sync.Mutex // bound calls can run concurrently
	notes  []Note
	nextID int
	native Native
}

// NewService returns a Service that reaches Wails through native.
func NewService(native Native) *Service {
	return &Service{native: native}
}

// ListNotes returns every note. A nil slice would reach JavaScript as null,
// so return an empty one.
func (s *Service) ListNotes() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Note{}, s.notes...)
}

// SaveNote validates and stores a note. A non-nil error rejects the
// frontend's promise.
func (s *Service) SaveNote(in NoteInput) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err
	}
	return s.store(Note(in))
}

// store adds a note, or replaces the one with the same ID, under s.mu and
// emits EventNotesChanged.
func (s *Service) store(note Note) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if note.ID == "" {
		s.nextID++
		note.ID = strconv.Itoa(s.nextID)
		s.notes = append(s.notes, note)
	} else {
		i := s.indexLocked(note.ID)
		if i < 0 {
			return Note{}, fmt.Errorf("%w: %q", ErrNotFound, note.ID)
		}
		s.notes[i] = note
	}
	s.native.Emit(EventNotesChanged, Changed{Count: len(s.notes)})
	return note, nil
}

// DeleteNote removes a note by ID.
func (s *Service) DeleteNote(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	s.notes = slices.Delete(s.notes, i, i+1)
	s.native.Emit(EventNotesChanged, Changed{Count: len(s.notes)})
	return nil
}

// ExportNotes asks for a file and writes the notes there as JSON. It returns
// the chosen path, or "" and a nil error when the user cancels. The context
// is the call's context: Wails cancels it when the frontend cancels the
// promise, and the generated binding does not expose it as a parameter.
func (s *Service) ExportNotes(ctx context.Context) (string, error) {
	path, err := s.native.SaveFile("notes.json")
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	if path == "" {
		return "", nil // cancelled: a normal outcome, nothing written
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	data, err := json.MarshalIndent(s.ListNotes(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	return path, nil
}

func (s *Service) indexLocked(id string) int {
	return slices.IndexFunc(s.notes, func(n Note) bool { return n.ID == id })
}
