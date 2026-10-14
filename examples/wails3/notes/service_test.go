package notes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeNative records events and answers the save dialog with a fixed path.
type fakeNative struct {
	mu     sync.Mutex
	path   string
	err    error
	events []Changed
}

func (f *fakeNative) SaveFile(string) (string, error) { return f.path, f.err }

func (f *fakeNative) Emit(event string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if event == EventNotesChanged {
		f.events = append(f.events, data.(Changed))
	}
}

func TestSaveNoteValidates(t *testing.T) {
	native := &fakeNative{}
	s := NewService(native)
	for _, in := range []NoteInput{
		{Title: "   "},
		{Title: strings.Repeat("x", maxTitle+1)},
		{Title: "ok", Body: strings.Repeat("x", maxBody+1)},
	} {
		if _, err := s.SaveNote(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("SaveNote(%.20q...) err = %v, want ErrInvalid", in.Title, err)
		}
	}
	if _, err := s.SaveNote(NoteInput{ID: "missing", Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown ID: err = %v", err)
	}
	if n := len(s.ListNotes()); n != 0 || len(native.events) != 0 {
		t.Fatalf("invalid input changed state: %d notes, %d events", n, len(native.events))
	}
}

func TestSaveAndDeleteEmitTypedEvents(t *testing.T) {
	native := &fakeNative{}
	s := NewService(native)
	n, err := s.SaveNote(NoteInput{Title: "  first  "})
	if err != nil || n.Title != "first" || n.ID == "" {
		t.Fatalf("SaveNote = %+v, %v", n, err)
	}
	if err := s.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNote(n.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	want := []Changed{{Count: 1}, {Count: 0}}
	if len(native.events) != 2 || native.events[0] != want[0] || native.events[1] != want[1] {
		t.Fatalf("events = %+v, want %+v", native.events, want)
	}
}

func TestListNotesIsNeverNil(t *testing.T) {
	b, _ := json.Marshal(NewService(&fakeNative{}).ListNotes())
	if string(b) != "[]" {
		t.Fatalf("got %s, want []", b)
	}
}

func TestExportCancelledWritesNothing(t *testing.T) {
	s := NewService(&fakeNative{path: ""})
	path, err := s.ExportNotes(context.Background())
	if path != "" || err != nil {
		t.Fatalf("cancelled export = %q, %v", path, err)
	}
}

func TestExportWritesJSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "notes.json")
	s := NewService(&fakeNative{path: out})
	if _, err := s.SaveNote(NoteInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ExportNotes(context.Background())
	if err != nil || got != out {
		t.Fatalf("export = %q, %v", got, err)
	}
	var notes []Note
	b, _ := os.ReadFile(out)
	if err := json.Unmarshal(b, &notes); err != nil || len(notes) != 1 {
		t.Fatalf("file = %s (%v)", b, err)
	}
}

func TestExportStopsWhenCallCancelled(t *testing.T) {
	out := filepath.Join(t.TempDir(), "notes.json")
	s := NewService(&fakeNative{path: out})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ExportNotes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("file written after cancel: %v", err)
	}
}

func TestDialogErrorIsReturned(t *testing.T) {
	s := NewService(&fakeNative{err: errors.New("no display")})
	if _, err := s.ExportNotes(context.Background()); err == nil || !strings.Contains(err.Error(), "no display") {
		t.Fatalf("err = %v", err)
	}
}

func TestConcurrentSaves(t *testing.T) {
	s := NewService(&fakeNative{})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := s.SaveNote(NoteInput{Title: "x"}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := len(s.ListNotes()); n != 50 {
		t.Fatalf("got %d notes, want 50", n)
	}
}
