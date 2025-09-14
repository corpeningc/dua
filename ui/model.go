package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
)

type LoadingCompleteMsg struct {
	Path string
	Success bool
	Error error
}

type BulkDeletionMsg struct {
	DeletedPaths []string
	SuccessCount int
	ErrorCount int
	Erros []error
}

// New message types for streaming updates
type StreamingUpdateMsg struct {
	UpdatedDir *scanner.StreamingDirInfo
}

type StreamingCompleteMsg struct {
	TotalFiles int64
	TotalDirs  int64
	TotalBytes int64
	Duration   time.Duration
	DirInfo    *scanner.DirInfo // Add the scanned directory data
}

type StreamingProgressMsg struct {
	Files int64
	Dirs  int64
	Bytes int64
}

// Real-time streaming message
type StreamingModelUpdateMsg struct {
	Model      *scanner.DirInfo
	Scanner    *scanner.RealTimeScanner
	Builder    *scanner.StreamingModelBuilder
	UpdateChan <-chan scanner.StreamUpdate
}

type SortMode int
const (
	SortByName SortMode = iota
	SortByDate
	SortBySize
	SortByType
)

func (s SortMode) String() string {
	switch s {
		case SortByName:
			return "Name"
		case SortByDate:
			return "Date"
		case SortBySize:
			return "Size"
		case SortByType:
			return "Type"
		default:
			return "Unknown"
	}
}

type Model struct {
	// Directory data
	rootDir *scanner.DirInfo
	currentPath string

	// Real-time streaming support
	realTimeScanner  *scanner.RealTimeScanner
	streamingBuilder *scanner.StreamingModelBuilder
	updateChan       <-chan scanner.StreamUpdate
	streamingMode    bool
	isScanning       bool
	scanStartTime    time.Time

	// Progress tracking
	progressFiles int64
	progressDirs  int64
	progressBytes int64

	// UI state
	cursor int // Which item is selected
	selected map[string]bool // Which items are selected
	expanded map[string]bool // Which directories are expanded
	markedForDeletion map[string]bool
	viewportTop int // First visible item index

	// Visual mode
	visualMode bool
	visualStart int // Anchor point for visual mode

	// Deletion mode
	deletionMode bool

	// Sorting state
	sortMode SortMode
	sortAsc bool

	// View state
	width int
	height int
}

func NewModel(rootDir *scanner.DirInfo, path string) Model {
	return Model {
		rootDir: rootDir,
		currentPath: path,

		cursor: 0,
		expanded: make(map[string]bool),
		selected: make(map[string]bool),
		viewportTop: 0,

		visualMode: false,
		visualStart: -1,

		width: 80,
		height: 24,

		sortMode: SortByName,
		sortAsc: false,

		streamingMode: false,
		isScanning: false,
	}
}

// NewStreamingModel creates a model with streaming support enabled
func NewStreamingModel(path string) Model {
	// Create immediate empty structure - NO blocking scan!
	rootDir := &scanner.DirInfo{
		Path: path,
		Size: 0,
		Files: make([]scanner.FileInfo, 0),
		Subdirs: make([]scanner.DirInfo, 0),
		IsLoaded: false,
		IsLoading: true,
		FileCount: 0,
		SubdirCount: 0,
	}

	model := Model {
		rootDir: rootDir,
		currentPath: path,
		streamingMode: true,
		isScanning: true, // Will start scanning after UI opens
		scanStartTime: time.Now(),

		cursor: 0,
		expanded: make(map[string]bool),
		selected: make(map[string]bool),
		viewportTop: 0,

		visualMode: false,
		visualStart: -1,

		width: 80,
		height: 24,

		sortMode: SortByName,
		sortAsc: false,
	}

	return model
}

	func (m Model) Init() tea.Cmd {
		// Start simple background loading after UI is open
		if m.streamingMode {
			return m.startSimpleStreaming()
		}
		return nil
	}

	// startSimpleStreaming does immediate directory listing then background size calc
	func (m Model) startSimpleStreaming() tea.Cmd {
		return func() tea.Msg {
			// Step 1: Get immediate directory structure (files and folders, no sizes)
			entries, err := os.ReadDir(m.currentPath)
			if err != nil {
				return LoadingCompleteMsg{
					Path: m.currentPath,
					Success: false,
					Error: err,
				}
			}

			// Create directory info with immediate listings
			dirInfo := &scanner.DirInfo{
				Path:    m.currentPath,
				Size:    0, // Will calculate in background
				Files:   make([]scanner.FileInfo, 0),
				Subdirs: make([]scanner.DirInfo, 0),
				IsLoaded: true,  // Mark as loaded so it shows content
				IsLoading: false,
			}

			// Add files (these are fast to get sizes for)
			for _, entry := range entries {
				if !entry.IsDir() {
					if info, err := entry.Info(); err == nil {
						dirInfo.Files = append(dirInfo.Files, scanner.FileInfo{
							Name: entry.Name(),
							Size: info.Size(),
						})
						dirInfo.Size += info.Size()
						dirInfo.FileCount++
					}
				}
			}

			// Add subdirectories (no sizes yet)
			for _, entry := range entries {
				if entry.IsDir() {
					fullPath := filepath.Join(m.currentPath, entry.Name())
					subdir := scanner.DirInfo{
						Path:        fullPath,
						Size:        0, // Will calculate later
						Files:       make([]scanner.FileInfo, 0),
						Subdirs:     make([]scanner.DirInfo, 0),
						IsLoaded:    false,
						IsLoading:   false,
						FileCount:   0,
						SubdirCount: 0,
					}
					dirInfo.Subdirs = append(dirInfo.Subdirs, subdir)
					dirInfo.SubdirCount++
				}
			}

			return StreamingCompleteMsg{
				TotalFiles: int64(dirInfo.FileCount),
				TotalDirs: int64(dirInfo.SubdirCount),
				TotalBytes: dirInfo.Size,
				DirInfo: dirInfo,
			}
		}
	}


	func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
		switch msg := msg.(type) {
			case tea.WindowSizeMsg:
				m.width = msg.Width
				m.height = msg.Height

			case LoadingCompleteMsg:
				dirInfo := m.findDirectoryInTree(m.rootDir, msg.Path)
				if dirInfo != nil {
					dirInfo.IsLoading = false
					if msg.Success {
						dirInfo.IsLoaded = true
					}
				}
			
			// Real-time streaming updates - simplified to avoid infinite loops
			case StreamingModelUpdateMsg:
				// Update model with streaming data
				if msg.Model != nil {
					m.rootDir = msg.Model
					m.expanded[msg.Model.Path] = true // Auto-expand root
					m.isScanning = false // Mark as complete
				}

			case StreamingProgressMsg:
				m.progressFiles = msg.Files
				m.progressDirs = msg.Dirs
				m.progressBytes = msg.Bytes

			case StreamingCompleteMsg:
				m.isScanning = false
				m.progressFiles = msg.TotalFiles
				m.progressDirs = msg.TotalDirs
				m.progressBytes = msg.TotalBytes

				// Update rootDir with scanned data
				if msg.DirInfo != nil {
					m.rootDir = msg.DirInfo
					// Auto-expand the root directory so user can see contents immediately
					m.expanded[msg.DirInfo.Path] = true
				}

			// Update tree
			case BulkDeletionMsg:
				for _, path := range msg.DeletedPaths {
					m.removeItemFromTree(path)
				}

				// Reset visual
				m.visualMode = false
				m.visualStart = - 1
				m.selected = make(map[string]bool)

				// Reset deletion
				m.deletionMode = false
				m.markedForDeletion = make(map[string]bool)

			case tea.KeyMsg:
				switch msg.String() {
				case "ctrl+c", "q":
					return m, tea.Quit
				case "up", "k":
					if m.cursor > 0 {
						m.cursor--
						if (m.visualMode) {
							m.updateVisualSelection()
						}
						m.adjustViewport()
					}
				case "down", "j":
					maxItems := m.countVisibleItems()
					if m.cursor < maxItems - 1 {
						m.cursor++
						if (m.visualMode) {
							m.updateVisualSelection()
						}
						m.adjustViewport()
					}
				case "right", "l", "enter":
					// Expand directory with lazy loading
					if path, isDir := m.getCurrentItem(); isDir && path != "" {
						m.expanded[path] = true
						// Trigger lazy loading if not already loaded
						return m, m.startAsyncLoading(path)
					}
				case "left", "h":
					// Collapse directory
					if path, isDir := m.getCurrentItem(); isDir && path != "" {
						m.expanded[path] = false
					}
				case "ctrl+s":
					m.sortAsc = !m.sortAsc
				case "s":
					m.sortMode = (m.sortMode + 1) % 4 // Cycle through sort modes
				case "esc":
					// Turn off visual mode and deletion mode
					m.visualMode = false
					m.visualStart = -1
					m.selected = make(map[string]bool)
					m.deletionMode = false
					m.markedForDeletion = make(map[string]bool)
				case "t":
					// Add current item to selected
					if path, _ := m.getCurrentItem(); path != "" {
						m.selected[path] = true
					}
				case "d":
					// Deletion mode already on
					if m.deletionMode {
						if len(m.markedForDeletion) > 0 {
							return m, m.performBulkDeletion()
						}
					} else {
						m.deletionMode = true
						m.markedForDeletion = make(map[string]bool)

						if m.visualMode && len(m.selected) > 0 {
							for path := range m.selected {
								m.markedForDeletion[path] = true
							}
						} else {
							if path, _ := m.getCurrentItem(); path != "" {
								m.markedForDeletion[path] = true
							}
						}
					}
				// Navigate to top
				case "g":
					m.cursor = 0
					if m.visualMode {
						m.updateVisualSelection()
					}
					m.adjustViewport()
				case "G":
					m.cursor = m.countVisibleItems() - 1
					if m.visualMode {
						m.updateVisualSelection()
					}
					m.adjustViewport()
				case "v":
					if m.visualMode {
						m.visualMode = false
						m.visualStart = -1
						m.selected = make(map[string]bool)
					} else {
						m.visualMode = true
						m.visualStart = m.cursor

						if path, _ := m.getCurrentItem(); path != "" {
							m.selected[path] = true
						}
					}
				}
		}
		return m, nil
	}

// adjustViewport ensures the cursor stays visible within terminal bounds
func (m *Model) adjustViewport() {
	// Reserve 4 lines for header (2) + footer (2)
	visibleLines := m.height - 4
	if visibleLines < 1 {
		visibleLines = 10 // Fallback for very small terminals
	}
	
	// Scroll down if cursor is below visible area
	if m.cursor >= m.viewportTop + visibleLines {
		m.viewportTop = m.cursor - visibleLines + 1
	}
	
	// Scroll up if cursor is above visible area  
	if m.cursor < m.viewportTop {
		m.viewportTop = m.cursor
	}
	
	// Don't scroll past the beginning
	if m.viewportTop < 0 {
		m.viewportTop = 0
	}
}

func (m Model) sortDirectoryContents(dir *scanner.DirInfo) ([]scanner.FileInfo, []scanner.DirInfo) {
	files := make([]scanner.FileInfo, len(dir.Files))
	copy(files, dir.Files)

	subdirs := make([]scanner.DirInfo, len(dir.Subdirs))
	copy(subdirs, dir.Subdirs)

	m.sortFiles(files)
	m.sortDirs(subdirs)

	return files, subdirs
}

func (m Model) sortFiles(files []scanner.FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		var result bool
		switch m.sortMode {
		case SortByName:
			result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		case SortBySize:
			result = files[i].Size < files[j].Size
		case SortByDate:
			// Need dates on file info
			result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		case SortByType:
			// get extensions
			extI := getFileExtension(files[i].Name)
			extJ := getFileExtension(files[j].Name)
			if extI == extJ {
				result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
			} else {
				result = strings.ToLower(extI) < strings.ToLower(extJ)
			}
		}

		if !m.sortAsc {
			result = !result
		}

		return result
	})
}

func (m Model) sortDirs(subdirs []scanner.DirInfo) {
	sort.Slice(subdirs, func(i, j int) bool {
		var result bool

		switch m.sortMode {
		case SortByName:
			nameI := getBaseName(subdirs[i].Path)
			nameJ := getBaseName(subdirs[j].Path)
			result = strings.ToLower(nameI) < strings.ToLower(nameJ)

		case SortBySize:
			result = subdirs[i].Size < subdirs[j].Size
		case SortByDate:
			nameI := getBaseName(subdirs[i].Path)
			nameJ := getBaseName(subdirs[j].Path)
			result = strings.ToLower(nameI) < strings.ToLower(nameJ)
		case SortByType:
			nameI := getBaseName(subdirs[i].Path)
			nameJ := getBaseName(subdirs[j].Path)
			result = strings.ToLower(nameI) < strings.ToLower(nameJ)
		}

		if !m.sortAsc {
			result = !result
		}

		return result
	})
}

func getFileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
					return parts[len(parts)-1]
	}
	return "" // No extension
}

func (m *Model) findDirectoryInTree (dir *scanner.DirInfo, targetPath string) *scanner.DirInfo {
	if dir.Path == targetPath {
	return dir
	}

	// Search in subdirectories
	for i := range dir.Subdirs {
		if found := m.findDirectoryInTree(&dir.Subdirs[i], targetPath); found != nil {
			return found
		}
	}
	return nil
}

func (m *Model) startAsyncLoading(path string) tea.Cmd {
	dirInfo := m.findDirectoryInTree(m.rootDir, path)
	if dirInfo != nil && !dirInfo.IsLoaded && !dirInfo.IsLoading {
		return loadDirectoryCmd(dirInfo)
	}
	return nil
}

func loadDirectoryCmd(dirInfo *scanner.DirInfo) tea.Cmd {
	return func() tea.Msg {
		err := scanner.LoadDirectoryContents(dirInfo)
		return LoadingCompleteMsg{
			Path: dirInfo.Path,
			Success: err == nil,
			Error: err,
		}
	}
}

func (m *Model) updateVisualSelection() {
	// Clear selected and recalculate range
	m.selected = make(map[string]bool)
	start := min(m.visualStart, m.cursor)
	end := max(m.visualStart, m.cursor)
	
	for i := start; i <= end; i++ {
		if path, _ := m.findItemAtIndex(m.rootDir, 0, 0, i); path != "" {
			m.selected[path] = true
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func (m Model) performBulkDeletion() tea.Cmd {
	pathsToDelete := make([]string, 0, len(m.markedForDeletion))

	for path := range m.markedForDeletion {
		pathsToDelete = append(pathsToDelete, path)
	}

	return func() tea.Msg {
		var errors []error
		var deletedPaths []string

		for _, path := range pathsToDelete {
			if err := os.RemoveAll(path); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", path, err))
			} else {
				deletedPaths = append(deletedPaths, path)
			}
		}

		return BulkDeletionMsg{
			DeletedPaths: deletedPaths,
			SuccessCount: len(deletedPaths),
			ErrorCount: len(errors),
			Erros: errors,
		}
	}
}

func (m *Model) removeItemFromTree(targetPath string) {
	parentPath := filepath.Dir(targetPath)

	if parent := m.findDirectoryInTree(m.rootDir, parentPath); parent != nil {
		for i, file := range parent.Files {
			if filepath.Join(parent.Path, file.Name) == targetPath {
				parent.Files = append(parent.Files[:i], parent.Files[i+1:]...)
				parent.Size -= file.Size
				break
			}
		}
		
		for i, subdir := range parent.Subdirs {
			if subdir.Path == targetPath {
				parent.Subdirs = append(parent.Subdirs[:i], parent.Subdirs[i+1:]...)
				parent.Size -= subdir.Size
				parent.SubdirCount--
				break
			}
		}

		m.updateParentSizes(parentPath)
	}
} 

func (m *Model) updateParentSizes(path string) {
	for path != "/" && path != "." {
		if dir := m.findDirectoryInTree(m.rootDir, path); dir != nil {
			var newSize int64
			for _, file := range dir.Files {
				newSize += file.Size
			}

			for _, subdir := range dir.Subdirs {
				newSize += subdir.Size
			}

			dir.Size = newSize
		}
		path = filepath.Dir(path)
	}
}

// View renders the current state
func (m Model) View() string {
	return m.ViewTree()
}