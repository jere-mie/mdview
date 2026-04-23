package watcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	targetPath string
	fileMode   bool
	watcher    *fsnotify.Watcher
	onChange   func(string)
}

func New(targetPath string, onChange func(string)) (*Watcher, error) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target: %w", err)
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		targetPath: filepath.Clean(targetPath),
		fileMode:   !info.IsDir(),
		watcher:    fsWatcher,
		onChange:   onChange,
	}

	if w.fileMode {
		if err := w.watcher.Add(filepath.Dir(w.targetPath)); err != nil {
			_ = w.watcher.Close()
			return nil, fmt.Errorf("watch parent directory: %w", err)
		}
	} else {
		if err := w.addRecursive(w.targetPath); err != nil {
			_ = w.watcher.Close()
			return nil, err
		}
	}

	go w.loop()
	return w, nil
}

func (w *Watcher) Close() error {
	return w.watcher.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
		return
	}

	eventPath := filepath.Clean(event.Name)
	if w.fileMode {
		if eventPath == w.targetPath && w.onChange != nil {
			w.onChange(eventPath)
		}
		return
	}

	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(eventPath); err == nil && info.IsDir() {
			_ = w.addRecursive(eventPath)
			return
		}
	}

	if w.onChange != nil {
		w.onChange(eventPath)
	}
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if err := w.watcher.Add(path); err != nil {
			return fmt.Errorf("watch directory %s: %w", path, err)
		}
		return nil
	})
}
