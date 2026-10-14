package behavior

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// rewrite replaces a file's contents with one write and no truncation, so a
// watcher never reads a half-written file.
func rewrite(t *testing.T, path, body string) {
	t.Helper()
	const size = 64
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt([]byte(body+"#"+strings.Repeat(" ", size-len(body)-1)), 0); err != nil {
		t.Fatal(err)
	}
}

// watch starts a watcher whose callback reports the port it sees. The
// callback runs on the watcher goroutine, so it reads the instance there.
func watch(t *testing.T, path string) <-chan int {
	t.Helper()
	v := viper.New()
	read(t, v, path)
	seen := make(chan int, 16)
	v.OnConfigChange(func(fsnotify.Event) { seen <- v.GetInt("port") })
	v.WatchConfig()
	return seen
}

func next(t *testing.T, seen <-chan int, wait time.Duration) (int, bool) {
	t.Helper()
	select {
	case port := <-seen:
		return port, true
	case <-time.After(wait):
		return 0, false
	}
}

// references/reload-and-write.md: the change hook runs after a write, and it
// still runs when the reread fails, with the old values left in place.
func TestWatchHookRunsOnFailedReread(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	rewrite(t, path, "port: 1\n")
	seen := watch(t, path)

	rewrite(t, path, "port: 2\n")
	if port, ok := next(t, seen, 5*time.Second); !ok || port != 2 {
		t.Fatalf("valid edit: port=%d ok=%v", port, ok)
	}
	rewrite(t, path, "port: [\n")
	if port, ok := next(t, seen, 5*time.Second); !ok || port != 2 {
		t.Fatalf("malformed edit: port=%d ok=%v; want the hook to run with the old value", port, ok)
	}
}

// references/reload-and-write.md: removing the file ends the watch for good.
func TestWatchEndsWhenFileRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	rewrite(t, path, "port: 1\n")
	seen := watch(t, path)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond) // let the watcher see the removal
	rewrite(t, path, "port: 2\n")
	if port, ok := next(t, seen, 2*time.Second); ok {
		t.Fatalf("hook ran after the file was removed: port=%d", port)
	}
}

// references/reload-and-write.md: WatchConfig returns nothing. When the
// watcher cannot be registered (here the directory does not exist), Viper
// logs the error and the hook never runs, even after the file appears.
// Source: viper.go WatchConfig, the watcher.Add error branch.
func TestWatchConfigLogsRegistrationFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(dir, "app.yaml")
	v := viper.New()
	v.SetConfigFile(path)
	seen := make(chan struct{}, 1)
	v.OnConfigChange(func(fsnotify.Event) { seen <- struct{}{} })
	v.WatchConfig()

	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	rewrite(t, path, "port: 1\n")
	select {
	case <-seen:
		t.Fatal("hook ran although the watcher was never registered")
	case <-time.After(time.Second):
	}
}
