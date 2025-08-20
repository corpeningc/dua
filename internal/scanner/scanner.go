package scanner

import (
	"fmt"
	"os"
	"path/filepath"
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

// ScanDirectoryLazy performs fast initial scan with sizes but no file enumeration
func ScanDirectoryLazy(path string) (*DirInfo, error) {
	dirInfo := &DirInfo{
		Path:      path,
		Files:     make([]FileInfo, 0),
		Subdirs:   make([]DirInfo, 0),
		IsLoaded:  false,
		IsLoading: false,
	}

	// Fast walk to calculate sizes and counts without storing individual items
	err := filepath.WalkDir(path, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible items
		}

		if walkPath == path {
			return nil // Skip root directory itself
		}

		// Get file info for size
		info, err := d.Info()
		if err != nil {
			return nil // Skip if can't get info
		}

		dirInfo.Size += info.Size()

		// Count direct children only (not deep traversal counts)
		if filepath.Dir(walkPath) == path {
			if d.IsDir() {
				dirInfo.SubdirCount++
			} else {
				dirInfo.FileCount++
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", path, err)
	}

	return dirInfo, nil
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

	for _, entry := range entries {
		fullPath := filepath.Join(dirInfo.Path, entry.Name())

		if entry.IsDir() {
			// Create lazy subdirectory (size will be calculated when needed)
			subdir, err := ScanDirectoryLazy(fullPath)
			if err != nil {
				continue // Skip directories we can't access
			}
			dirInfo.Subdirs = append(dirInfo.Subdirs, *subdir)
		} else {
			// Add file information
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