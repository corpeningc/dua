package scanner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// StreamUpdate represents a real-time update from the scanner
type StreamUpdate struct {
	Type     string // "file", "dir", "size_update", "complete"
	Path     string
	Name     string
	Size     int64
	IsDir    bool
	Parent   string
	Depth    int
	Error    error
}

// RealTimeScanner performs true streaming directory scanning
type RealTimeScanner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	updateChan chan StreamUpdate
	wg         sync.WaitGroup
	maxDepth   int
	maxWorkers int
}

// NewRealTimeScanner creates a new streaming scanner
func NewRealTimeScanner() *RealTimeScanner {
	ctx, cancel := context.WithCancel(context.Background())
	return &RealTimeScanner{
		ctx:        ctx,
		cancel:     cancel,
		updateChan: make(chan StreamUpdate, 1000), // Large buffer for smooth streaming
		maxDepth:   10,                            // Reasonable recursion limit
		maxWorkers: runtime.NumCPU() * 2,          // Parallelism for I/O
	}
}

// StartStreaming begins streaming directory scan
func (s *RealTimeScanner) StartStreaming(rootPath string) <-chan StreamUpdate {
	go s.streamDirectory(rootPath, "", 0)
	return s.updateChan
}

// Stop cancels the streaming operation
func (s *RealTimeScanner) Stop() {
	s.cancel()
	s.wg.Wait()
	close(s.updateChan)
}

// streamDirectory recursively streams directory contents
func (s *RealTimeScanner) streamDirectory(path, parent string, depth int) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Check for cancellation
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Don't go too deep to prevent infinite recursion
		if depth > s.maxDepth {
			return
		}

		// Read directory entries
		entries, err := os.ReadDir(path)
		if err != nil {
			s.updateChan <- StreamUpdate{
				Type:   "error",
				Path:   path,
				Parent: parent,
				Depth:  depth,
				Error:  err,
			}
			return
		}

		// Send immediate directory structure (no sizes yet)
		var files []os.DirEntry
		var dirs []os.DirEntry

		for _, entry := range entries {
			// Check for cancellation frequently
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			if entry.IsDir() {
				dirs = append(dirs, entry)
				// Send directory immediately with placeholder size
				s.updateChan <- StreamUpdate{
					Type:   "dir",
					Path:   filepath.Join(path, entry.Name()),
					Name:   entry.Name(),
					Size:   0, // Will be calculated later
					IsDir:  true,
					Parent: path,
					Depth:  depth + 1,
				}
			} else {
				files = append(files, entry)
			}
		}

		// Send files immediately with sizes (files are fast)
		for _, file := range files {
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			s.updateChan <- StreamUpdate{
				Type:   "file",
				Path:   filepath.Join(path, file.Name()),
				Name:   file.Name(),
				Size:   info.Size(),
				IsDir:  false,
				Parent: path,
				Depth:  depth + 1,
			}
		}

		// Recursively stream subdirectories in parallel (controlled)
		if len(dirs) > 0 {
			s.streamSubdirectoriesParallel(path, dirs, depth)
		}

		// After processing all entries, calculate and send directory size
		s.calculateAndSendDirectorySize(path, parent, depth)
	}()
}

// streamSubdirectoriesParallel processes subdirectories with controlled parallelism
func (s *RealTimeScanner) streamSubdirectoriesParallel(parentPath string, dirs []os.DirEntry, depth int) {
	// Create semaphore to limit concurrent goroutines
	sem := make(chan struct{}, s.maxWorkers)

	for _, dir := range dirs {
		// Check for cancellation
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Acquire semaphore
		sem <- struct{}{}

		go func(d os.DirEntry) {
			defer func() { <-sem }() // Release semaphore

			fullPath := filepath.Join(parentPath, d.Name())
			s.streamDirectory(fullPath, parentPath, depth+1)
		}(dir)
	}

	// Wait for all subdirectories to complete by filling the semaphore
	for i := 0; i < s.maxWorkers; i++ {
		sem <- struct{}{}
	}
}

// calculateAndSendDirectorySize calculates total size and sends update
func (s *RealTimeScanner) calculateAndSendDirectorySize(path, parent string, depth int) {
	// Quick calculation of direct files only (non-recursive)
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	var totalSize int64
	for _, entry := range entries {
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}

	// Send size update
	s.updateChan <- StreamUpdate{
		Type:   "size_update",
		Path:   path,
		Size:   totalSize,
		Parent: parent,
		Depth:  depth,
	}
}

// StreamingModelBuilder builds directory model from streaming updates
type StreamingModelBuilder struct {
	root    *DirInfo
	pathMap map[string]*DirInfo // Quick lookup
	mu      sync.RWMutex
}

// NewStreamingModelBuilder creates a builder for streaming updates
func NewStreamingModelBuilder(rootPath string) *StreamingModelBuilder {
	root := &DirInfo{
		Path:    rootPath,
		Size:    0,
		Files:   make([]FileInfo, 0),
		Subdirs: make([]DirInfo, 0),
		IsLoaded: false,
		IsLoading: true,
	}

	return &StreamingModelBuilder{
		root:    root,
		pathMap: map[string]*DirInfo{rootPath: root},
	}
}

// ProcessUpdate processes a streaming update
func (b *StreamingModelBuilder) ProcessUpdate(update StreamUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch update.Type {
	case "file":
		b.addFile(update)
	case "dir":
		b.addDirectory(update)
	case "size_update":
		b.updateDirectorySize(update)
	}
}

// addFile adds a file to the model
func (b *StreamingModelBuilder) addFile(update StreamUpdate) {
	parent := b.findOrCreateParent(update.Parent)

	file := FileInfo{
		Name: update.Name,
		Size: update.Size,
	}

	parent.Files = append(parent.Files, file)
	parent.Size += update.Size
	parent.FileCount++
}

// addDirectory adds a directory to the model
func (b *StreamingModelBuilder) addDirectory(update StreamUpdate) {
	parent := b.findOrCreateParent(update.Parent)

	subdir := &DirInfo{
		Path:        update.Path,
		Size:        update.Size,
		Files:       make([]FileInfo, 0),
		Subdirs:     make([]DirInfo, 0),
		IsLoaded:    false,
		IsLoading:   true,
		FileCount:   0,
		SubdirCount: 0,
	}

	parent.Subdirs = append(parent.Subdirs, *subdir)
	parent.SubdirCount++
	b.pathMap[update.Path] = subdir
}

// updateDirectorySize updates directory size
func (b *StreamingModelBuilder) updateDirectorySize(update StreamUpdate) {
	if dir, exists := b.pathMap[update.Path]; exists {
		dir.Size = update.Size
		dir.IsLoading = false
		dir.IsLoaded = true
	}
}

// findOrCreateParent finds or creates parent directory
func (b *StreamingModelBuilder) findOrCreateParent(parentPath string) *DirInfo {
	if dir, exists := b.pathMap[parentPath]; exists {
		return dir
	}

	// Should not happen in normal streaming, but handle gracefully
	parent := &DirInfo{
		Path:    parentPath,
		Files:   make([]FileInfo, 0),
		Subdirs: make([]DirInfo, 0),
	}
	b.pathMap[parentPath] = parent
	return parent
}

// GetSnapshot returns current model snapshot
func (b *StreamingModelBuilder) GetSnapshot() *DirInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.root
}

// CreateStreamingCmd creates a Bubble Tea command for streaming
func CreateStreamingCmd(rootPath string) tea.Cmd {
	return func() tea.Msg {
		scanner := NewRealTimeScanner()
		builder := NewStreamingModelBuilder(rootPath)

		updateChan := scanner.StartStreaming(rootPath)

		// Process some initial updates quickly
		timeout := time.After(50 * time.Millisecond) // Quick initial update
		processed := 0

		for processed < 100 { // Limit initial batch
			select {
			case update := <-updateChan:
				builder.ProcessUpdate(update)
				processed++
			case <-timeout:
				// Send what we have so far
				return StreamingModelUpdate{
					Model:   builder.GetSnapshot(),
					Scanner: scanner,
					Builder: builder,
					UpdateChan: updateChan,
				}
			}
		}

		return StreamingModelUpdate{
			Model:   builder.GetSnapshot(),
			Scanner: scanner,
			Builder: builder,
			UpdateChan: updateChan,
		}
	}
}

// StreamingModelUpdate message for Bubble Tea
type StreamingModelUpdate struct {
	Model      *DirInfo
	Scanner    *RealTimeScanner
	Builder    *StreamingModelBuilder
	UpdateChan <-chan StreamUpdate
}