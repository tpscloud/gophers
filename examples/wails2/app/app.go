package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Native is the part of the Wails runtime the app uses. Production code gets
// it from the context passed to OnStartup; tests pass a fake.
type Native interface {
	SaveFileDialog(runtime.SaveDialogOptions) (string, error)
	Emit(event string, data ...any)
}

// EventNotesChanged carries the full note list after every change.
const EventNotesChanged = "notes:changed"

// App is bound to the frontend. Each exported method becomes a function in
// wailsjs/go/app/App.js that returns a Promise.
type App struct {
	mu     sync.Mutex // bound calls run concurrently on Linux and macOS
	notes  []Note
	nextID int
	native Native
}

// New returns an App. Pass nil to use the Wails runtime once OnStartup runs.
func New(native Native) *App {
	return &App{native: native}
}

// NewOptions builds the Wails options. It is a function, not a method on
// App, because every exported method of a bound struct is callable from the
// frontend.
func NewOptions(a *App, assets fs.FS) *options.App {
	return &options.App{
		Title:       "Notes",
		Width:       960,
		Height:      640,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   a.startup,
		Bind:        []any{a},
	}
}

// startup keeps the Wails context. Runtime calls need this context; any other
// context makes the runtime exit the process.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.native == nil {
		a.native = wailsNative{ctx: ctx}
	}
}

// ListNotes returns every note. A nil slice would reach JavaScript as null,
// so return an empty slice.
func (a *App) ListNotes() []Note {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.listLocked()
}

// SaveNote validates and stores a note. A non-nil error rejects the
// frontend's Promise with the error text.
func (a *App) SaveNote(in NoteInput) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err
	}
	return a.store(Note(in))
}

// store adds a note, or replaces the one with the same ID, under a.mu and
// emits EventNotesChanged.
func (a *App) store(note Note) (Note, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if note.ID == "" {
		a.nextID++
		note.ID = strconv.Itoa(a.nextID)
		a.notes = append(a.notes, note)
	} else {
		i := a.indexLocked(note.ID)
		if i < 0 {
			return Note{}, fmt.Errorf("%w: %q", ErrNotFound, note.ID)
		}
		a.notes[i] = note
	}
	a.emitLocked()
	return note, nil
}

// DeleteNote removes a note by ID.
func (a *App) DeleteNote(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	i := a.indexLocked(id)
	if i < 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	a.notes = slices.Delete(a.notes, i, i+1)
	a.emitLocked()
	return nil
}

// ExportNotes asks for a file and writes the notes there as JSON. It returns
// the chosen path, or "" with a nil error when the user cancels the dialog.
func (a *App) ExportNotes() (string, error) {
	a.mu.Lock()
	native := a.native
	a.mu.Unlock()
	if native == nil {
		return "", fmt.Errorf("export: the app has not started")
	}
	path, err := native.SaveFileDialog(runtime.SaveDialogOptions{
		Title:           "Export notes",
		DefaultFilename: "notes.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	if path == "" {
		return "", nil // cancelled: a normal outcome, nothing written
	}
	data, err := json.MarshalIndent(a.ListNotes(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	return path, nil
}

func (a *App) listLocked() []Note {
	return append([]Note{}, a.notes...)
}

func (a *App) indexLocked(id string) int {
	return slices.IndexFunc(a.notes, func(n Note) bool { return n.ID == id })
}

func (a *App) emitLocked() {
	if a.native != nil {
		a.native.Emit(EventNotesChanged, a.listLocked())
	}
}

// wailsNative calls the Wails runtime with the context from OnStartup.
type wailsNative struct{ ctx context.Context }

func (n wailsNative) SaveFileDialog(o runtime.SaveDialogOptions) (string, error) {
	return runtime.SaveFileDialog(n.ctx, o)
}

func (n wailsNative) Emit(event string, data ...any) {
	runtime.EventsEmit(n.ctx, event, data...)
}
