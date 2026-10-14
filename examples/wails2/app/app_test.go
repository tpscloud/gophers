package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// fakeNative records events and returns a fixed dialog result.
type fakeNative struct {
	mu     sync.Mutex
	path   string
	err    error
	events []string
}

func (f *fakeNative) SaveFileDialog(runtime.SaveDialogOptions) (string, error) {
	return f.path, f.err
}

func (f *fakeNative) Emit(event string, _ ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func TestInvalidInputIsRejectedBeforeAnyEffect(t *testing.T) {
	native := &fakeNative{}
	a := New(native)
	for _, in := range []NoteInput{
		{Title: "   "},
		{Title: strings.Repeat("x", 121)},
		{Title: "big", Body: strings.Repeat("x", MaxBodyBytes+1)},
	} {
		if _, err := a.SaveNote(in); !errors.Is(err, ErrInvalidNote) {
			t.Errorf("SaveNote(%.20q) err = %v, want ErrInvalidNote", in.Title, err)
		}
	}
	if len(a.ListNotes()) != 0 || len(native.events) != 0 {
		t.Fatalf("invalid input changed state: notes=%v events=%v", a.ListNotes(), native.events)
	}
}

func TestSaveUpdateDelete(t *testing.T) {
	native := &fakeNative{}
	a := New(native)
	n, err := a.SaveNote(NoteInput{Title: " First "})
	if err != nil || n.ID == "" || n.Title != "First" {
		t.Fatalf("create: %+v, %v", n, err)
	}
	if _, err := a.SaveNote(NoteInput{ID: n.ID, Title: "Renamed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveNote(NoteInput{ID: "nope", Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := a.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteNote(n.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if got := strings.Join(native.events, ","); got != "notes:changed,notes:changed,notes:changed" {
		t.Fatalf("events = %s", got)
	}
}

// An empty list must reach JavaScript as [] rather than null.
func TestEmptyListIsJSONArray(t *testing.T) {
	b, err := json.Marshal(New(nil).ListNotes())
	if err != nil || string(b) != "[]" {
		t.Fatalf("got %s, %v", b, err)
	}
}

func TestNoteJSONFieldNames(t *testing.T) {
	b, _ := json.Marshal(Note{ID: "1", Title: "t", Body: "b"})
	if string(b) != `{"id":"1","title":"t","body":"b"}` {
		t.Fatalf("got %s", b)
	}
}

func TestExportCancelledWritesNothing(t *testing.T) {
	a := New(&fakeNative{path: ""})
	path, err := a.ExportNotes()
	if path != "" || err != nil {
		t.Fatalf("cancel: %q, %v", path, err)
	}
}

func TestExportWritesChosenFile(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "notes.json")
	a := New(&fakeNative{path: dst})
	if _, err := a.SaveNote(NoteInput{Title: "One"}); err != nil {
		t.Fatal(err)
	}
	path, err := a.ExportNotes()
	if err != nil || path != dst {
		t.Fatalf("export: %q, %v", path, err)
	}
	var got []Note
	b, _ := os.ReadFile(dst)
	if err := json.Unmarshal(b, &got); err != nil || len(got) != 1 || got[0].Title != "One" {
		t.Fatalf("file: %s, %v", b, err)
	}
}

func TestExportDialogErrorIsReturned(t *testing.T) {
	a := New(&fakeNative{err: errors.New("no display")})
	if _, err := a.ExportNotes(); err == nil || !strings.Contains(err.Error(), "no display") {
		t.Fatalf("err = %v", err)
	}
}

func TestExportBeforeStartupFails(t *testing.T) {
	if _, err := New(nil).ExportNotes(); err == nil {
		t.Fatal("want an error before OnStartup")
	}
}

// startup stores a runtime adapter around the Wails context and keeps an
// injected Native.
func TestStartupKeepsLifecycleContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "wails")
	a := New(nil)
	a.startup(ctx)
	n, ok := a.native.(wailsNative)
	if !ok || n.ctx.Value(key{}) != "wails" {
		t.Fatalf("native = %#v", a.native)
	}
	injected := &fakeNative{}
	b := New(injected)
	b.startup(ctx)
	if b.native != injected {
		t.Fatal("startup replaced an injected Native")
	}
}

func TestNewOptions(t *testing.T) {
	a := New(nil)
	o := NewOptions(a, os.DirFS("."))
	if len(o.Bind) != 1 || o.Bind[0] != a {
		t.Fatalf("Bind = %#v", o.Bind)
	}
	if o.OnStartup == nil || o.AssetServer == nil || o.AssetServer.Assets == nil {
		t.Fatal("OnStartup and AssetServer.Assets must be set")
	}
}

// Wails runs each bound call on its own goroutine on Linux and macOS, so
// calls overlap. Run with -race to check the locking.
func TestConcurrentCalls(t *testing.T) {
	a := New(&fakeNative{})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := a.SaveNote(NoteInput{Title: "n"}); err != nil {
				t.Error(err)
			}
			a.ListNotes()
		})
	}
	wg.Wait()
	if got := len(a.ListNotes()); got != 20 {
		t.Fatalf("got %d notes", got)
	}
}
