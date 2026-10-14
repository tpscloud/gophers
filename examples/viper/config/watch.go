package config

import (
	"errors"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Watch calls apply with a new snapshot each time the config file changes,
// or report when the edited file does not load. The callbacks run on Viper's
// watcher goroutine, so apply must publish safely (an atomic.Pointer, a
// mutex, or a channel send).
//
// The watching instance is never read. Each change runs Load on a fresh
// instance instead, because Viper calls the change hook even when its own
// reread failed, and that instance then still holds the old values.
//
// Watch pins src to the file it found, so a reload never switches to a file
// created later in a higher-priority directory while notifications still
// come from the old one. It loads that file once before watching, because
// WatchConfig returns nothing: it logs a failure to register the watcher and
// carries on. A nil error means the file loaded, not that the watcher is
// guaranteed to be running.
//
// Viper has no way to stop a watcher; it ends when the file is removed.
func Watch(src Source, apply func(Config), report func(error)) error {
	path, err := resolve(src)
	if err != nil {
		return err
	}
	if path == "" {
		return errors.New("no config file to watch")
	}
	src.File, src.Dirs = path, nil
	if _, err := Load(src); err != nil {
		return err
	}
	w := viper.New()
	w.SetConfigFile(path)
	w.OnConfigChange(func(fsnotify.Event) {
		cfg, err := Load(src)
		if err != nil {
			report(err)
			return
		}
		apply(cfg)
	})
	w.WatchConfig()
	return nil
}
