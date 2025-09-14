package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// DirInfo represents a directory with size information and lazy loading support.
type DirInfo struct {
	Path        string
	Size        int64
	Files       []FileInfo
	Subdirs     []DirInfo
	IsLoaded    bool
	IsLoading   bool
	FileCount   int
	SubdirCount int
}

// FileInfo represents a file with its name and size.
type FileInfo struct {
	Name string
	Size int64
}

// ScanDirectoryLazy performs fast parallel directory scanning with size calculation.
// Uses lazy loading approach - calculates directory sizes without enumerating contents.
func ScanDirectoryLazy(path string) (*DirInfo, error) {
	maxWorkers := runtime.NumCPU() * 4
	sem := make(chan struct{}, maxWorkers)
	return scanDirectoryParallel(path, sem)
}

func scanDirectoryParallel(path string, sem chan struct{}) (*DirInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	var files []os.DirEntry
	var dirs []os.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	var totalSize int64
	for _, file := range files {
		if info, err := file.Info(); err == nil {
			totalSize += info.Size()
		}
	}

	if len(dirs) > 0 {
		sizeChan := make(chan int64, len(dirs))
		dirChan := make(chan os.DirEntry, len(dirs))
		var wg sync.WaitGroup

		for _, dir := range dirs {
			dirChan <- dir
		}
		close(dirChan)

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
						sizeChan <- 0
					}
				}
			}()
		}

		go func() {
			wg.Wait()
			close(sizeChan)
		}()

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

// LoadDirectoryContents loads files and subdirectories for a directory with lazy loading.
func LoadDirectoryContents(dirInfo *DirInfo) error {
	if dirInfo.IsLoaded || dirInfo.IsLoading {
		return nil
	}

	dirInfo.IsLoading = true
	defer func() { dirInfo.IsLoading = false }()

	entries, err := os.ReadDir(dirInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirInfo.Path, err)
	}

	dirInfo.Files = make([]FileInfo, 0)
	dirInfo.Subdirs = make([]DirInfo, 0)

	var filesToProcess []os.DirEntry
	var dirsToProcess []os.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			dirsToProcess = append(dirsToProcess, entry)
		} else {
			filesToProcess = append(filesToProcess, entry)
		}
	}

	for _, entry := range filesToProcess {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name: entry.Name(),
			Size: info.Size(),
		}
		dirInfo.Files = append(dirInfo.Files, fileInfo)
	}

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
					return
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

