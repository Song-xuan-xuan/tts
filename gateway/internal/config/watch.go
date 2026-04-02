package config

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func Watch(ctx context.Context, logger *slog.Logger, store *Store, path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != filepath.Clean(path) {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				if err := store.Reload(); err != nil {
					logger.Error("config reload failed", "error", err)
					continue
				}
				logger.Info("config reloaded", "path", path)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("config watcher error", "error", err)
			}
		}
	}()

	return watcher.Add(filepath.Dir(path))
}
