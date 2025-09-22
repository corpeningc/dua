package scanner

import (
	"fmt"
	"os"
	"path/filepath"
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

func LoadDirectoryContents(dir *DirInfo) error {
	// Already loading
	if dir.IsLoaded || dir.IsLoading {
		return nil
	}

	dir.IsLoading = true
	defer func() {dir.IsLoading = false}()

	entries, err := os.ReadDir(dir.Path)

	if err != nil {
		return fmt.Errorf("Error reading directory %s: %v\n", dir.Path, err)
	}

	// Append directories and files to this DirInfo
	for _, entry := range entries {
		if entry.IsDir() {
			fullPath := filepath.Join(dir.Path, entry.Name())
			subdir := DirInfo {
				Path: fullPath,
				Size: 0,
				Files: []FileInfo{},
				Subdirs: []DirInfo{},
				IsLoaded: false,
				IsLoading: false,
				FileCount: 0,
				SubdirCount: 0,
			}

			dir.Subdirs = append(dir.Subdirs, subdir)
			dir.SubdirCount++
		} else {
			if info, err := entry.Info(); err == nil {
				file := FileInfo {
					Name: entry.Name(),
					Size: info.Size(),
				}

				dir.Files = append(dir.Files, file)
				dir.FileCount++
				dir.Size += info.Size()
			}
		}
	}

	dir.IsLoaded = true
	return nil
}