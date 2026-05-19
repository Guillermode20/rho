package agentutils

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileWatcher watches files and directories for changes with debounced notifications.
type FileWatcher struct {
	paths    map[string]time.Time
	interval time.Duration
	debounce time.Duration
	mu       sync.Mutex
	onChange func(path string)
	stopCh   chan struct{}
	timers   map[string]*time.Timer
}

// NewFileWatcher creates a new file watcher with the given check interval and debounce.
func NewFileWatcher(interval, debounce time.Duration) *FileWatcher {
	return &FileWatcher{
		paths:    make(map[string]time.Time),
		interval: interval,
		debounce: debounce,
		stopCh:   make(chan struct{}),
		timers:   make(map[string]*time.Timer),
	}
}

// Watch adds a file or directory to watch.
func (fw *FileWatcher) Watch(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	fw.mu.Lock()
	fw.paths[path] = info.ModTime()
	fw.mu.Unlock()
	return nil
}

// Unwatch removes a path from watching.
func (fw *FileWatcher) Unwatch(path string) {
	fw.mu.Lock()
	delete(fw.paths, path)
	fw.mu.Unlock()
}

// OnChange sets the callback for file changes.
func (fw *FileWatcher) OnChange(fn func(path string)) {
	fw.onChange = fn
}

// Start begins polling for changes.
func (fw *FileWatcher) Start() {
	go func() {
		ticker := time.NewTicker(fw.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fw.checkChanges()
			case <-fw.stopCh:
				return
			}
		}
	}()
}

// Stop stops watching.
func (fw *FileWatcher) Stop() {
	close(fw.stopCh)
}

func (fw *FileWatcher) checkChanges() {
	fw.mu.Lock()
	paths := make([]string, 0, len(fw.paths))
	for p := range fw.paths {
		paths = append(paths, p)
	}
	fw.mu.Unlock()

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		fw.mu.Lock()
		oldModTime := fw.paths[path]
		newModTime := info.ModTime()
		fw.paths[path] = newModTime
		fw.mu.Unlock()

		if newModTime.After(oldModTime) {
			fw.debounceNotify(path)
		}
	}
}

func (fw *FileWatcher) debounceNotify(path string) {
	fw.mu.Lock()
	if t, ok := fw.timers[path]; ok {
		t.Stop()
	}
	fw.timers[path] = time.AfterFunc(fw.debounce, func() {
		fw.mu.Lock()
		delete(fw.timers, path)
		fw.mu.Unlock()
		if fw.onChange != nil {
			fw.onChange(path)
		}
	})
	fw.mu.Unlock()
}

// WatchDir watches a directory recursively for changes.
func WatchDir(root string, onChange func(path string)) (*FileWatcher, error) {
	fw := NewFileWatcher(500*time.Millisecond, 200*time.Millisecond)
	fw.OnChange(onChange)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		return fw.Watch(path)
	})
	if err != nil {
		return nil, err
	}

	fw.Start()
	return fw, nil
}

// WatchFiles watches a set of specific files for changes.
func WatchFiles(paths []string, onChange func(path string)) *FileWatcher {
	fw := NewFileWatcher(500*time.Millisecond, 200*time.Millisecond)
	fw.OnChange(onChange)
	for _, p := range paths {
		fw.Watch(p)
	}
	fw.Start()
	return fw
}
