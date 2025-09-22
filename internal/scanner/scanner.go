package scanner

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

