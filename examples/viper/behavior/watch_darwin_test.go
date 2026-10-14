package behavior

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// references/reload-and-write.md: on macOS, replacing the file by rename (an
// atomic save) is reported as a removal, and the watch ends.
func TestWatchEndsOnRenameSaveDarwin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	rewrite(t, path, "port: 1\n")
	seen := watch(t, path)

	tmp := filepath.Join(dir, ".app.yaml.tmp")
	rewrite(t, tmp, "port: 2\n")
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	rewrite(t, path, "port: 3\n")
	if port, ok := next(t, seen, 2*time.Second); ok {
		t.Fatalf("hook ran after a rename save: port=%d", port)
	}
}
