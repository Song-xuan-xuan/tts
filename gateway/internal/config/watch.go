package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type watchEvent struct {
	Name string
	Op   fsnotify.Op
}

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
				if !shouldReloadOnEvent(path, watchEvent{Name: event.Name, Op: event.Op}) {
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

func shouldReloadOnEvent(path string, event watchEvent) bool {
	cleanPath := filepath.Clean(path)
	cleanDir := filepath.Dir(cleanPath)
	cleanEvent := filepath.Clean(event.Name)

	if filepath.Dir(cleanEvent) != cleanDir {
		return false
	}

	if cleanEvent == cleanPath {
		return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0
	}

	if event.Op&fsnotify.Rename != 0 {
		return true
	}

	if event.Op&fsnotify.Create != 0 {
		if _, err := os.Stat(cleanPath); err == nil {
			return true
		}
	}

	return false
}
