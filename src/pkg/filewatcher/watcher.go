package filewatcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of file system event
type EventType int

const (
	// EventCreated indicates a file or directory was created
	EventCreated EventType = iota
	// EventModified indicates a file or directory was modified
	EventModified
	// EventDeleted indicates a file or directory was deleted
	EventDeleted
	// EventRenamed indicates a file or directory was renamed
	EventRenamed
)

// Event represents a file system event with additional context
type Event struct {
	Type      EventType
	Path      string
	IsDir     bool
	Time      time.Time
	OldPath   string // Only for rename events
	Extension string
}

// Config contains configuration for the file watcher
type Config struct {
	DebounceTime     time.Duration
	ScanInterval     time.Duration
	MaxEventQueue    int
	IgnoreDotFiles   bool
	IgnoreGitChanges bool
	IncludePatterns  []string
	ExcludePatterns  []string
}

// DefaultConfig returns default configuration for the file watcher
func DefaultConfig() *Config {
	return &Config{
		DebounceTime:     500 * time.Millisecond,
		ScanInterval:     10 * time.Minute,
		MaxEventQueue:    1000,
		IgnoreDotFiles:   true,
		IgnoreGitChanges: true,
		IncludePatterns:  []string{"*.md"},
		ExcludePatterns:  []string{".git/*", ".obsidian/*"},
	}
}

// Watcher monitors directories for file system events
type Watcher struct {
	config       *Config
	watcher      *fsnotify.Watcher
	events       chan Event
	directories  map[string]bool
	debounceMap  map[string]*time.Timer
	debounceMu   sync.Mutex
	recentEvents map[string]time.Time
	recentMu     sync.RWMutex
	done         chan struct{}
}

// NewWatcher creates a new file watcher
func NewWatcher(config *Config) (*Watcher, error) {
	// Create fsnotify watcher
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Watcher{
		config:       config,
		watcher:      fswatcher,
		events:       make(chan Event, config.MaxEventQueue),
		directories:  make(map[string]bool),
		debounceMap:  make(map[string]*time.Timer),
		recentEvents: make(map[string]time.Time),
		done:         make(chan struct{}),
	}, nil
}

// Start begins monitoring directories for changes
func (w *Watcher) Start(ctx context.Context) (<-chan Event, error) {
	// Start event processing
	go w.processEvents(ctx)

	// Start periodic full scan
	go w.periodicScan(ctx)

	return w.events, nil
}

// processEvents handles events from fsnotify
func (w *Watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case evt, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFsEvent(evt)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// periodicScan performs a full scan periodically
func (w *Watcher) periodicScan(ctx context.Context) {
	ticker := time.NewTicker(w.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case <-ticker.C:
			w.scanDirectories()
		}
	}
}

// scanDirectories scans all watched directories for changes
func (w *Watcher) scanDirectories() {
	w.debounceMu.Lock()
	dirs := make([]string, 0, len(w.directories))
	for dir := range w.directories {
		dirs = append(dirs, dir)
	}
	w.debounceMu.Unlock()

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip if error
			}

			// Check if file should be processed
			if !w.shouldProcess(path, info.IsDir()) {
				if info.IsDir() && w.isExcludedDir(path) {
					return filepath.SkipDir
				}
				return nil
			}

			// Emit event for files not recently processed
			w.recentMu.RLock()
			lastEvent, exists := w.recentEvents[path]
			w.recentMu.RUnlock()

			if !exists || time.Since(lastEvent) > w.config.ScanInterval {
				w.queueEvent(EventModified, path, info.IsDir(), "")
			}

			return nil
		})

		if err != nil {
			log.Printf("Error scanning directory %s: %v", dir, err)
		}
	}
}

// handleFsEvent processes fsnotify events and translates them to our Event type
func (w *Watcher) handleFsEvent(evt fsnotify.Event) {
	// Get file info
	info, err := os.Stat(evt.Name)
	isDir := false
	exists := !os.IsNotExist(err)

	if err == nil {
		isDir = info.IsDir()
	}

	// Skip if we shouldn't process this file
	if !w.shouldProcess(evt.Name, isDir) {
		return
	}

	// Determine event type
	var eventType EventType
	switch {
	case evt.Op&fsnotify.Create == fsnotify.Create:
		eventType = EventCreated
		if exists && isDir {
			w.watchDirectory(evt.Name)
		}
	case evt.Op&fsnotify.Write == fsnotify.Write:
		eventType = EventModified
	case evt.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = EventDeleted
		if isDir {
			w.unwatchDirectory(evt.Name)
		}
	case evt.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = EventRenamed
		if isDir {
			w.unwatchDirectory(evt.Name)
		}
	default:
		return // Ignore other events
	}

	// Debounce and queue the event
	w.debounceEvent(eventType, evt.Name, isDir, "")
}

// shouldProcess determines if a file should be monitored
func (w *Watcher) shouldProcess(path string, isDir bool) bool {
	// Always process directories (for watching)
	if isDir {
		return !w.isExcludedDir(path)
	}

	// Check if path matches exclude patterns
	for _, pattern := range w.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return false
		}

		// Check if path is within excluded directory
		if strings.HasSuffix(pattern, "/*") {
			dirPattern := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, dirPattern) {
				return false
			}
		}
	}

	// Check for dot files
	if w.config.IgnoreDotFiles && strings.HasPrefix(filepath.Base(path), ".") {
		return false
	}

	// Check if path matches include patterns
	for _, pattern := range w.config.IncludePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}

	// If no include patterns, we'll include all non-excluded files
	return len(w.config.IncludePatterns) == 0
}

// isExcludedDir checks if a directory should be excluded
func (w *Watcher) isExcludedDir(path string) bool {
	// Check .git directory
	if w.config.IgnoreGitChanges && (strings.Contains(path, "/.git/") || strings.HasSuffix(path, "/.git")) {
		return true
	}

	// Check for dot directories
	if w.config.IgnoreDotFiles && strings.HasPrefix(filepath.Base(path), ".") {
		return true
	}

	// Check explicit exclude patterns
	for _, pattern := range w.config.ExcludePatterns {
		if strings.HasSuffix(pattern, "/*") {
			dirPattern := strings.TrimSuffix(pattern, "/*")
			if path == dirPattern || strings.HasPrefix(path, dirPattern+"/") {
				return true
			}
		}
	}

	return false
}

// debounceEvent waits for the debounce period before queueing an event
func (w *Watcher) debounceEvent(eventType EventType, path string, isDir bool, oldPath string) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Cancel any existing timer
	if timer, exists := w.debounceMap[path]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[path] = time.AfterFunc(w.config.DebounceTime, func() {
		w.debounceMu.Lock()
		delete(w.debounceMap, path)
		w.debounceMu.Unlock()

		w.queueEvent(eventType, path, isDir, oldPath)
	})
}

// queueEvent sends an event to the event channel
func (w *Watcher) queueEvent(eventType EventType, path string, isDir bool, oldPath string) {
	// Update recent events
	w.recentMu.Lock()
	w.recentEvents[path] = time.Now()
	w.recentMu.Unlock()

	// Create and send event
	event := Event{
		Type:      eventType,
		Path:      path,
		IsDir:     isDir,
		Time:      time.Now(),
		OldPath:   oldPath,
		Extension: strings.ToLower(filepath.Ext(path)),
	}

	select {
	case w.events <- event:
		// Event sent successfully
	default:
		// Channel is full
		log.Printf("Warning: event queue is full, discarding event for %s", path)
	}
}

// watchDirectory adds a directory to the watch list
func (w *Watcher) watchDirectory(path string) error {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Check if already watching
	if _, exists := w.directories[path]; exists {
		return nil
	}

	// Add to fsnotify watcher
	if err := w.watcher.Add(path); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", path, err)
	}

	// Add to our tracked directories
	w.directories[path] = true

	// Watch subdirectories recursively
	return filepath.Walk(path, func(subpath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() && subpath != path {
			// Skip excluded directories
			if w.isExcludedDir(subpath) {
				return filepath.SkipDir
			}

			// Add to watcher
			if watchErr := w.watcher.Add(subpath); watchErr != nil {
				log.Printf("Error watching subdirectory %s: %v", subpath, watchErr)
				return nil // Continue with other directories
			}

			w.directories[subpath] = true
		}

		return nil
	})
}

// unwatchDirectory removes a directory from the watch list
func (w *Watcher) unwatchDirectory(path string) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Check if we're watching this directory
	if _, exists := w.directories[path]; !exists {
		return
	}

	// Remove from fsnotify
	_ = w.watcher.Remove(path)

	// Remove from our tracked directories
	delete(w.directories, path)

	// Remove any subdirectories
	for dir := range w.directories {
		if strings.HasPrefix(dir, path+"/") {
			_ = w.watcher.Remove(dir)
			delete(w.directories, dir)
		}
	}
}

// AddPath adds a path to the watch list
func (w *Watcher) AddPath(path string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	// If it's a directory, watch it
	if info.IsDir() {
		return w.watchDirectory(absPath)
	}

	// If it's a file, watch the parent directory
	return w.watchDirectory(filepath.Dir(absPath))
}

// Close stops the watcher and releases resources
func (w *Watcher) Close() error {
	close(w.done)

	// Cancel all pending timers
	w.debounceMu.Lock()
	for _, timer := range w.debounceMap {
		timer.Stop()
	}
	w.debounceMap = nil
	w.debounceMu.Unlock()

	// Close the fsnotify watcher
	if err := w.watcher.Close(); err != nil {
		return fmt.Errorf("error closing watcher: %w", err)
	}

	return nil
}

// GetWatchedDirectories returns a list of watched directories
func (w *Watcher) GetWatchedDirectories() []string {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	dirs := make([]string, 0, len(w.directories))
	for dir := range w.directories {
		dirs = append(dirs, dir)
	}
	return dirs
}
