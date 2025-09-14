package scanner

import (
	"path/filepath"
	"sync"
	"time"
)

// StreamingDirInfo represents a directory that builds up over time
type StreamingDirInfo struct {
	Path        string
	Size        int64
	Files       map[string]*StreamingFileInfo // Map for O(1) updates
	Subdirs     map[string]*StreamingDirInfo  // Map for O(1) updates
	IsLoaded    bool
	IsLoading   bool
	LastUpdate  time.Time
	FileCount   int
	SubdirCount int
	Depth       int
	mu          sync.RWMutex // Protect concurrent access
}

// StreamingFileInfo represents file information
type StreamingFileInfo struct {
	Name string
	Size int64
	Path string
}

// NewStreamingDirInfo creates a new streaming directory info
func NewStreamingDirInfo(path string, depth int) *StreamingDirInfo {
	return &StreamingDirInfo{
		Path:       path,
		Files:      make(map[string]*StreamingFileInfo),
		Subdirs:    make(map[string]*StreamingDirInfo),
		IsLoading:  true,
		LastUpdate: time.Now(),
		Depth:      depth,
	}
}

// AddFile adds or updates a file entry
func (d *StreamingDirInfo) AddFile(name string, size int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	fullPath := filepath.Join(d.Path, name)

	if existing, exists := d.Files[name]; exists {
		// Update existing file
		d.Size -= existing.Size
		d.Size += size
		existing.Size = size
	} else {
		// Add new file
		d.Files[name] = &StreamingFileInfo{
			Name: name,
			Size: size,
			Path: fullPath,
		}
		d.Size += size
		d.FileCount++
	}

	d.LastUpdate = time.Now()
}

// AddSubdir adds or updates a subdirectory entry
func (d *StreamingDirInfo) AddSubdir(name string, size int64) *StreamingDirInfo {
	d.mu.Lock()
	defer d.mu.Unlock()

	fullPath := filepath.Join(d.Path, name)

	if existing, exists := d.Subdirs[name]; exists {
		// Update existing directory size
		d.Size -= existing.Size
		d.Size += size
		existing.Size = size
		existing.LastUpdate = time.Now()
		return existing
	} else {
		// Add new subdirectory
		subdir := NewStreamingDirInfo(fullPath, d.Depth+1)
		subdir.Size = size
		d.Subdirs[name] = subdir
		d.Size += size
		d.SubdirCount++
		d.LastUpdate = time.Now()
		return subdir
	}
}

// UpdateSize updates the directory size (usually from size calculations)
func (d *StreamingDirInfo) UpdateSize(newSize int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Size = newSize
	d.LastUpdate = time.Now()
}

// GetFiles returns a snapshot of current files (thread-safe)
func (d *StreamingDirInfo) GetFiles() []StreamingFileInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	files := make([]StreamingFileInfo, 0, len(d.Files))
	for _, file := range d.Files {
		files = append(files, *file)
	}
	return files
}

// GetSubdirs returns a snapshot of current subdirectories (thread-safe)
func (d *StreamingDirInfo) GetSubdirs() []*StreamingDirInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	subdirs := make([]*StreamingDirInfo, 0, len(d.Subdirs))
	for _, subdir := range d.Subdirs {
		// Create a new copy without mutex to avoid lock copying
		subdir.mu.RLock()
		subdirCopy := &StreamingDirInfo{
			Path:        subdir.Path,
			Size:        subdir.Size,
			IsLoaded:    subdir.IsLoaded,
			IsLoading:   subdir.IsLoading,
			LastUpdate:  subdir.LastUpdate,
			FileCount:   subdir.FileCount,
			SubdirCount: subdir.SubdirCount,
			Depth:       subdir.Depth,
			Files:       make(map[string]*StreamingFileInfo),
			Subdirs:     make(map[string]*StreamingDirInfo),
		}
		subdir.mu.RUnlock()
		subdirs = append(subdirs, subdirCopy)
	}
	return subdirs
}

// MarkComplete marks the directory as fully loaded
func (d *StreamingDirInfo) MarkComplete() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.IsLoading = false
	d.IsLoaded = true
	d.LastUpdate = time.Now()
}

// GetStats returns current statistics
func (d *StreamingDirInfo) GetStats() (fileCount, subdirCount int, totalSize int64, lastUpdate time.Time) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.FileCount, d.SubdirCount, d.Size, d.LastUpdate
}

// FindSubdir finds a subdirectory by path (thread-safe)
func (d *StreamingDirInfo) FindSubdir(path string) *StreamingDirInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Direct child
	for _, subdir := range d.Subdirs {
		if subdir.Path == path {
			return subdir
		}
	}

	// Recursive search in subdirs
	for _, subdir := range d.Subdirs {
		if found := subdir.FindSubdir(path); found != nil {
			return found
		}
	}

	return nil
}

// ConvertToLegacy converts StreamingDirInfo to legacy DirInfo format
func (d *StreamingDirInfo) ConvertToLegacy() *DirInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	legacy := &DirInfo{
		Path:        d.Path,
		Size:        d.Size,
		IsLoaded:    d.IsLoaded,
		IsLoading:   d.IsLoading,
		FileCount:   d.FileCount,
		SubdirCount: d.SubdirCount,
		Files:       make([]FileInfo, 0, len(d.Files)),
		Subdirs:     make([]DirInfo, 0, len(d.Subdirs)),
	}

	// Convert files
	for _, file := range d.Files {
		legacy.Files = append(legacy.Files, FileInfo{
			Name: file.Name,
			Size: file.Size,
		})
	}

	// Convert subdirs recursively
	for _, subdir := range d.Subdirs {
		legacySubdir := subdir.ConvertToLegacy()
		legacy.Subdirs = append(legacy.Subdirs, *legacySubdir)
	}

	return legacy
}

// StreamingDirManager manages the streaming directory tree
type StreamingDirManager struct {
	root    *StreamingDirInfo
	scanner *StreamingScanner
	mu      sync.RWMutex
	updates chan *StreamingDirInfo // Channel for UI updates
}

// NewStreamingDirManager creates a new streaming directory manager
func NewStreamingDirManager(rootPath string) *StreamingDirManager {
	return &StreamingDirManager{
		root:    NewStreamingDirInfo(rootPath, 0),
		scanner: NewStreamingScanner(),
		updates: make(chan *StreamingDirInfo, 100),
	}
}

// StartScanning begins the streaming scan process
func (m *StreamingDirManager) StartScanning() <-chan *StreamingDirInfo {
	go m.processStreamingResults()
	return m.updates
}

// processStreamingResults processes streaming scan results and updates the tree
func (m *StreamingDirManager) processStreamingResults() {
	defer close(m.updates)

	resultChan := m.scanner.ScanDirectory(m.root.Path)

	for result := range resultChan {
		switch result.Type {
		case "file":
			m.handleFileResult(result)
		case "dir":
			m.handleDirResult(result)
		case "dir_size_update":
			m.handleDirSizeUpdate(result)
		case "progress":
			m.handleProgressUpdate(result)
		case "error":
			// Handle error (could send error updates)
			continue
		}

		// Send update to UI
		m.updates <- m.findOrCreateDir(filepath.Dir(result.Path))
	}
}

// handleFileResult processes a file scan result
func (m *StreamingDirManager) handleFileResult(result StreamingScanResult) {
	parentPath := filepath.Dir(result.Path)
	parentDir := m.findOrCreateDir(parentPath)
	parentDir.AddFile(result.Name, result.Size)
}

// handleDirResult processes a directory scan result
func (m *StreamingDirManager) handleDirResult(result StreamingScanResult) {
	parentPath := filepath.Dir(result.Path)
	parentDir := m.findOrCreateDir(parentPath)
	parentDir.AddSubdir(result.Name, result.Size)
}

// handleDirSizeUpdate processes a directory size update
func (m *StreamingDirManager) handleDirSizeUpdate(result StreamingScanResult) {
	dir := m.findOrCreateDir(result.Path)
	dir.UpdateSize(result.Size)
}

// handleProgressUpdate processes progress updates
func (m *StreamingDirManager) handleProgressUpdate(result StreamingScanResult) {
	// Could emit progress events for UI progress bars
}

// findOrCreateDir finds or creates a directory in the tree
func (m *StreamingDirManager) findOrCreateDir(path string) *StreamingDirInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == m.root.Path {
		return m.root
	}

	// Try to find existing directory
	if found := m.root.FindSubdir(path); found != nil {
		return found
	}

	// Create path if it doesn't exist
	return m.createDirPath(path)
}

// createDirPath creates the full directory path
func (m *StreamingDirManager) createDirPath(path string) *StreamingDirInfo {
	// This is a simplified implementation
	// In reality, you'd need to create the full path hierarchy
	parentPath := filepath.Dir(path)
	dirName := filepath.Base(path)

	var parentDir *StreamingDirInfo
	if parentPath == m.root.Path {
		parentDir = m.root
	} else {
		parentDir = m.createDirPath(parentPath)
	}

	return parentDir.AddSubdir(dirName, 0)
}

// GetRoot returns the current root directory
func (m *StreamingDirManager) GetRoot() *StreamingDirInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root
}

// Stop stops the scanning process
func (m *StreamingDirManager) Stop() {
	m.scanner.Stop()
}