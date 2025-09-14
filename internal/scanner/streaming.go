package scanner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// StreamingScanResult represents a single scan update
type StreamingScanResult struct {
	Path        string
	Type        string // "file", "dir", "error", "progress"
	Size        int64
	Name        string
	Error       error
	Depth       int
	TotalFiles  int64 // Running count
	TotalDirs   int64 // Running count
	BytesScanned int64 // Running total
}

// StreamingScanner provides real-time directory scanning with instant UI updates
type StreamingScanner struct {
	maxWorkers   int
	batchSize    int
	updateDelay  time.Duration

	// Counters for progress tracking
	totalFiles   int64
	totalDirs    int64
	bytesScanned int64

	// Control channels
	ctx          context.Context
	cancel       context.CancelFunc
	resultChan   chan StreamingScanResult
	done         chan struct{}
}

// NewStreamingScanner creates a scanner optimized for real-time UI updates
func NewStreamingScanner() *StreamingScanner {
	ctx, cancel := context.WithCancel(context.Background())

	return &StreamingScanner{
		maxWorkers:  runtime.NumCPU() * 4, // More workers for I/O bound operations
		batchSize:   100,                  // Send updates in batches
		updateDelay: 50 * time.Millisecond, // Smooth UI updates
		ctx:         ctx,
		cancel:      cancel,
		resultChan:  make(chan StreamingScanResult, 1000), // Large buffer
		done:        make(chan struct{}),
	}
}

// ScanDirectory starts streaming scan - returns immediately with result channel
func (s *StreamingScanner) ScanDirectory(rootPath string) <-chan StreamingScanResult {
	go s.scanWithStreaming(rootPath)
	return s.resultChan
}

// Stop cancels the scanning operation
func (s *StreamingScanner) Stop() {
	s.cancel()
	<-s.done // Wait for cleanup
}

// scanWithStreaming performs the actual streaming scan
func (s *StreamingScanner) scanWithStreaming(rootPath string) {
	defer close(s.resultChan)
	defer close(s.done)

	// Start with root directory immediate listing
	if err := s.scanDirectoryLevel(rootPath, 0); err != nil {
		s.resultChan <- StreamingScanResult{
			Path:  rootPath,
			Type:  "error",
			Error: err,
		}
		return
	}

	// Send final progress update
	s.resultChan <- StreamingScanResult{
		Type:         "progress",
		TotalFiles:   atomic.LoadInt64(&s.totalFiles),
		TotalDirs:    atomic.LoadInt64(&s.totalDirs),
		BytesScanned: atomic.LoadInt64(&s.bytesScanned),
	}
}

// scanDirectoryLevel scans a single directory level with immediate results
func (s *StreamingScanner) scanDirectoryLevel(path string, depth int) error {
	// Check for cancellation
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Separate files and directories for optimal processing
	var files []os.DirEntry
	var dirs []os.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Process files immediately (fast operation)
	s.processFiles(path, files, depth)

	// Process directories with controlled parallelism
	if len(dirs) > 0 {
		s.processDirectories(path, dirs, depth)
	}

	return nil
}

// processFiles handles file entries with immediate streaming results
func (s *StreamingScanner) processFiles(parentPath string, files []os.DirEntry, depth int) {
	batch := make([]StreamingScanResult, 0, s.batchSize)

	for _, file := range files {
		// Check for cancellation
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		info, err := file.Info()
		if err != nil {
			continue // Skip inaccessible files
		}

		size := info.Size()
		atomic.AddInt64(&s.totalFiles, 1)
		atomic.AddInt64(&s.bytesScanned, size)

		result := StreamingScanResult{
			Path:  filepath.Join(parentPath, file.Name()),
			Type:  "file",
			Size:  size,
			Name:  file.Name(),
			Depth: depth,
		}

		batch = append(batch, result)

		// Send batch when full
		if len(batch) >= s.batchSize {
			s.sendBatch(batch)
			batch = batch[:0] // Reset batch
		}
	}

	// Send remaining files in batch
	if len(batch) > 0 {
		s.sendBatch(batch)
	}
}

// processDirectories handles directory entries with worker pool
func (s *StreamingScanner) processDirectories(parentPath string, dirs []os.DirEntry, depth int) {
	// Send immediate directory entries (no size calculation yet)
	for _, dir := range dirs {
		atomic.AddInt64(&s.totalDirs, 1)

		s.resultChan <- StreamingScanResult{
			Path:  filepath.Join(parentPath, dir.Name()),
			Type:  "dir",
			Size:  0, // Will be calculated later
			Name:  dir.Name(),
			Depth: depth,
		}
	}

	// Don't recurse too deep automatically - let UI control expansion
	if depth >= 2 {
		return
	}

	// Use worker pool for deeper scanning
	dirChan := make(chan os.DirEntry, len(dirs))
	var wg sync.WaitGroup

	// Limit concurrent workers
	workers := s.maxWorkers
	if workers > len(dirs) {
		workers = len(dirs)
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.directoryWorker(parentPath, dirChan, depth)
		}()
	}

	// Send work to workers
	for _, dir := range dirs {
		select {
		case dirChan <- dir:
		case <-s.ctx.Done():
			close(dirChan)
			wg.Wait()
			return
		}
	}

	close(dirChan)
	wg.Wait()
}

// directoryWorker processes directories in parallel
func (s *StreamingScanner) directoryWorker(parentPath string, dirChan <-chan os.DirEntry, depth int) {
	for dir := range dirChan {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		fullPath := filepath.Join(parentPath, dir.Name())

		// Quick size calculation for this directory only
		if size, err := s.calculateDirectorySize(fullPath); err == nil {
			s.resultChan <- StreamingScanResult{
				Path: fullPath,
				Type: "dir_size_update",
				Size: size,
				Name: dir.Name(),
				Depth: depth,
			}
		}

		// Optionally recurse based on depth
		if depth < 3 { // Limit recursion depth
			s.scanDirectoryLevel(fullPath, depth+1)
		}
	}
}

// calculateDirectorySize calculates size for immediate directory only (non-recursive)
func (s *StreamingScanner) calculateDirectorySize(path string) (int64, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}

	var size int64
	for _, entry := range entries {
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				size += info.Size()
			}
		}
	}

	return size, nil
}

// sendBatch sends a batch of results with progress updates
func (s *StreamingScanner) sendBatch(batch []StreamingScanResult) {
	for _, result := range batch {
		select {
		case s.resultChan <- result:
		case <-s.ctx.Done():
			return
		}
	}

	// Send progress update
	s.resultChan <- StreamingScanResult{
		Type:         "progress",
		TotalFiles:   atomic.LoadInt64(&s.totalFiles),
		TotalDirs:    atomic.LoadInt64(&s.totalDirs),
		BytesScanned: atomic.LoadInt64(&s.bytesScanned),
	}
}

// GetProgress returns current scanning progress
func (s *StreamingScanner) GetProgress() (files, dirs, bytes int64) {
	return atomic.LoadInt64(&s.totalFiles),
		   atomic.LoadInt64(&s.totalDirs),
		   atomic.LoadInt64(&s.bytesScanned)
}