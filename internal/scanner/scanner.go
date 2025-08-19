package scanner

import (
	"fmt"
	"os"
	"path/filepath"
)

type DirInfo struct {
	Path string
	Size int64
	Files []FileInfo
	Subdirs []DirInfo
}

type FileInfo struct {
	Name string
	Size int64
}

func ScanDirectory(path string) (*DirInfo, error) {
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
			subdir, err := ScanDirectory(fullPath)

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