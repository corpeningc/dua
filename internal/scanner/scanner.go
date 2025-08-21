package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type DirInfo struct {
	Path        string
	Size        int64
	Files       []FileInfo
	Subdirs     []DirInfo
	IsLoaded    bool  // Whether files/subdirs have been enumerated
	IsLoading   bool  // Whether currently loading contents
	FileCount   int   // Number of files (known even when not loaded)
	SubdirCount int   // Number of subdirs (known even when not loaded)
}

type FileInfo struct {
	Name string
	Size int64
}

// ScanDirectoryLazy performs fast parallel scan with sizes but no file enumeration
func ScanDirectoryLazy(path string) (*DirInfo, error) {
	// Limit goroutines to prevent resource exhaustion
	maxWorkers := runtime.NumCPU() * 2
	sem := make(chan struct{}, maxWorkers)
	return scanDirectoryParallel(path, sem)
}

// scanDirectoryParallel performs parallel directory size calculation
func scanDirectoryParallel(path string, sem chan struct{}) (*DirInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	// Separate files from directories
	var files []os.DirEntry
	var dirs []os.DirEntry
	
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	// Process files sequentially 
	var totalSize int64
	for _, file := range files {
		if info, err := file.Info(); err == nil {
			totalSize += info.Size()
		}
	}

	// Process directories in parallel using worker pool
	if len(dirs) > 0 {
		sizeChan := make(chan int64, len(dirs))
		dirChan := make(chan os.DirEntry, len(dirs))
		var wg sync.WaitGroup
		
		// Send all directories to work channel
		for _, dir := range dirs {
			dirChan <- dir
		}
		close(dirChan)
		
		// Start worker goroutines
		maxWorkers := cap(sem)
		if maxWorkers > len(dirs) {
			maxWorkers = len(dirs)
		}
		
		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				for d := range dirChan {
					subpath := filepath.Join(path, d.Name())
					if subInfo, err := scanDirectoryParallel(subpath, sem); err == nil {
						sizeChan <- subInfo.Size
					} else {
						sizeChan <- 0 // Skip inaccessible directories
					}
				}
			}()
		}
		
		go func() {
			wg.Wait()
			close(sizeChan)
		}()
		
		// Collect results
		for size := range sizeChan {
			totalSize += size
		}
	}

	return &DirInfo{
		Path:        path,
		Size:        totalSize,
		Files:       make([]FileInfo, 0),
		Subdirs:     make([]DirInfo, 0),
		IsLoaded:    false,
		IsLoading:   false,
		FileCount:   len(files),
		SubdirCount: len(dirs),
	}, nil
}

// LoadDirectoryContents loads the actual files and subdirectories for a directory
func LoadDirectoryContents(dirInfo *DirInfo) error {
	if dirInfo.IsLoaded || dirInfo.IsLoading {
		return nil // Already loaded or loading
	}

	dirInfo.IsLoading = true
	defer func() { dirInfo.IsLoading = false }()

	entries, err := os.ReadDir(dirInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirInfo.Path, err)
	}

	// Clear existing slices and rebuild
	dirInfo.Files = make([]FileInfo, 0)
	dirInfo.Subdirs = make([]DirInfo, 0)

	// Separate files and directories for concurrent processing
	var filesToProcess []os.DirEntry
	var dirsToProcess []os.DirEntry
	
	for _, entry := range entries {
		if entry.IsDir() {
			dirsToProcess = append(dirsToProcess, entry)
		} else {
			filesToProcess = append(filesToProcess, entry)
		}
	}

	// Process files sequentially (fast)
	for _, entry := range filesToProcess {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't access
		}

		fileInfo := FileInfo{
			Name: entry.Name(),
			Size: info.Size(),
		}
		dirInfo.Files = append(dirInfo.Files, fileInfo)
	}

	// Process directories concurrently (can be slow)
	if len(dirsToProcess) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex
		subdirs := make([]DirInfo, 0, len(dirsToProcess))

		for _, entry := range dirsToProcess {
			wg.Add(1)
			go func(entry os.DirEntry) {
				defer wg.Done()
				
				fullPath := filepath.Join(dirInfo.Path, entry.Name())
				subdir, err := ScanDirectoryLazy(fullPath)
				if err != nil {
					return // Skip directories we can't access
				}

				mu.Lock()
				subdirs = append(subdirs, *subdir)
				mu.Unlock()
			}(entry)
		}
		
		wg.Wait()
		dirInfo.Subdirs = subdirs
	}

	dirInfo.IsLoaded = true
	return nil
}

// ScanDirectoryFull performs full recursive scan (original function, kept for compatibility)
func ScanDirectoryFull(path string) (*DirInfo, error) {
	dirInfo := &DirInfo{
		Path: path,
		Files: make([]FileInfo, 0),
		Subdirs: make([]DirInfo, 0),
	}

	entries, err := os.ReadDir(path)

	if err != nil{
		return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	for _, entry:= range entries {
		fullPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			// Recursively scan subdirectory
			subdir, err := ScanDirectoryFull(fullPath)

			if err != nil {
				// Skip directories we cant read or access
				continue
			}

			dirInfo.Subdirs = append(dirInfo.Subdirs, *subdir)
			dirInfo.Size += subdir.Size
		} else {
			// Get file information
			info, err := entry.Info()
			if err != nil {
				continue
			}

			fileInfo := FileInfo{
				Name: entry.Name(),
				Size: info.Size(),
			}

			dirInfo.Files = append(dirInfo.Files, fileInfo)
			dirInfo.Size += fileInfo.Size
		}
	}

	return dirInfo, nil
}