package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
)

// LoadingCompleteMsg indicates that a directory has finished loading.
type LoadingCompleteMsg struct {
	Path    string
	Success bool
	Error   error
}

// BulkDeletionMsg reports the results of a bulk deletion operation.
type BulkDeletionMsg struct {
	DeletedPaths []string
	SuccessCount int
	ErrorCount   int
	Errors       []error
}

type StreamingUpdateMsg struct {
	Update scanner.StreamingUpdate
	UpdateChan <-chan scanner.StreamingUpdate
	ErrorChan <-chan error
}

type StreamErrorMsg struct {
	Error error
}

// SortMode defines different ways to sort directory contents.
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

// Model represents the application state for the directory viewer.
type Model struct {
	rootDir     *scanner.DirInfo
	currentPath string

	streamingScanner *scanner.StreamingScanner
	directoryMap     map[string]*scanner.DirInfo
	updateChan			<-chan scanner.StreamingUpdate
	errorChan			<-chan error
	isScanning    bool
	scanStartTime time.Time

	progressFiles int
	progressDirs  int
	progressBytes int64

	cursor            int
	selected          map[string]bool
	expanded          map[string]bool
	markedForDeletion map[string]bool
	viewportTop       int

	visualMode  bool
	visualStart int

	deletionMode bool

	sortMode SortMode
	sortAsc  bool

	width  int
	height int
}

// NewModel creates a new model for the directory viewer.
func NewModel(rootDir *scanner.DirInfo, path string) Model {
	return Model{
		rootDir:     rootDir,
		currentPath: path,
		cursor:      0,
		expanded:    make(map[string]bool),
		selected:    make(map[string]bool),
		viewportTop: 0,
		visualMode:  false,
		visualStart: -1,
		width:       80,
		height:      24,
		sortMode:    SortByName,
		sortAsc:     false,
	}
}

// NewStreamingModel creates a model with fast startup and progressive loading.
func NewStreamingModel(path string) Model {
	rootDir := &scanner.DirInfo{
		Path:        path,
		Size:        0,
		Files:       make([]scanner.FileInfo, 0),
		Subdirs:     make([]scanner.DirInfo, 0),
		IsLoaded:    false,
		IsLoading:   true,
		FileCount:   0,
		SubdirCount: 0,
	}

	return Model{
		rootDir:          rootDir,
		currentPath:      path,
		streamingScanner: scanner.NewStreamingScanner(),
		directoryMap:     make(map[string]*scanner.DirInfo),
		isScanning:       true,
		scanStartTime:    time.Now(),
		cursor:           0,
		expanded:         make(map[string]bool),
		selected:         make(map[string]bool),
		viewportTop:      0,
		visualMode:       false,
		visualStart:      -1,
		width:            80,
		height:           24,
		sortMode:         SortByName,
		sortAsc:          false,
	}
}

// Init initializes the model, starting background loading if in streaming mode.
func (m Model) Init() tea.Cmd {
	return m.startConcurrentStreaming()
}

func (m Model) startConcurrentStreaming() tea.Cmd {
	updateChan, errorChan := m.streamingScanner.StartStreaming(m.currentPath)

	return tea.Batch(
		m.listenForUpdates(updateChan, errorChan),
		m.listenForErrors(errorChan),
	)
}

func (m Model) listenForUpdates(updateChan <-chan scanner.StreamingUpdate, errorChan <-chan error) tea.Cmd {
	return func() tea.Msg {
		log.Printf("DEBUG: listenForUpdates waiting for update...")
		update := <-updateChan
		log.Printf("DEBUG: listenForUpdates received update for: %s", update.Path)
		return StreamingUpdateMsg{
			Update: update,
			UpdateChan: updateChan,
			ErrorChan: errorChan,
		}
	}
}

func (m Model) listenForErrors(errorChan <-chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-errorChan
		return StreamErrorMsg{Error: err}
	}
}


// Update handles all messages and user input for the directory viewer.
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

	case StreamingUpdateMsg:
		log.Printf("DEBUG: Received StreamingUpdateMsg")
		update := msg.Update
		if update.IsComplete {
			log.Printf("DEBUG: Scan complete")
			m.isScanning = false
			if m.streamingScanner != nil {
				m.streamingScanner.Stop()
			}
		} else {
			log.Printf("DEBUG: Processing update for path: %s, files: %d, dirs: %d", update.Path, update.FileCount, update.DirCount)
			// Process incremental update
			m.progressFiles += update.FileCount
			m.progressDirs += update.DirCount
			m.progressBytes += update.TotalSize

			if update.DirInfo != nil {
				log.Printf("DEBUG: Storing directory in map: %s", update.DirInfo.Path)
				m.directoryMap[update.DirInfo.Path] = update.DirInfo

				if update.Path == m.currentPath {
					log.Printf("DEBUG: Setting root directory: %s", update.Path)
					m.rootDir = update.DirInfo
					m.expanded[update.Path] = true
				} else {
					// Integrate this directory into the tree structure
					log.Printf("DEBUG: Integrating directory into tree: %s", update.DirInfo.Path)
					m.integrateDirectoryIntoTree(update.DirInfo)
				}
			}
		}
		log.Printf("DEBUG: Returning batch with listeners")
		return m, tea.Batch(
			m.listenForUpdates(msg.UpdateChan, msg.ErrorChan),
			m.listenForErrors(msg.ErrorChan),
		)

	case BulkDeletionMsg:
		for _, path := range msg.DeletedPaths {
			m.removeItemFromTree(path)
		}

		m.visualMode = false
		m.visualStart = -1
		m.selected = make(map[string]bool)

		m.deletionMode = false
		m.markedForDeletion = make(map[string]bool)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.visualMode {
					m.updateVisualSelection()
				}
				m.adjustViewport()
			}
		case "down", "j":
			maxItems := m.countVisibleItems()
			if m.cursor < maxItems-1 {
				m.cursor++
				if m.visualMode {
					m.updateVisualSelection()
				}
				m.adjustViewport()
			}
		case "right", "l", "enter":
			if path, isDir := m.getCurrentItem(); isDir && path != "" {
				log.Printf("DEBUG: Attempting to expand directory: %s", path)
				if _, exists := m.directoryMap[path]; exists {
					log.Printf("DEBUG: Directory found in map, expanding: %s", path)
					m.expanded[path] = true
				} else {
					log.Printf("DEBUG: Directory not in map yet: %s", path)
					// Still expand it - it will show loading
					m.expanded[path] = true
				}
			}
		case "left", "h":
			if path, isDir := m.getCurrentItem(); isDir && path != "" {
				m.expanded[path] = false
			}
		case "ctrl+s":
			m.sortAsc = !m.sortAsc
		case "s":
			m.sortMode = (m.sortMode + 1) % 4
		case "esc":
			m.visualMode = false
			m.visualStart = -1
			m.selected = make(map[string]bool)
			m.deletionMode = false
			m.markedForDeletion = make(map[string]bool)
		case "t":
			if path, _ := m.getCurrentItem(); path != "" {
				m.selected[path] = true
			}
		case "d":
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

// adjustViewport ensures the cursor stays visible within terminal bounds.
func (m *Model) adjustViewport() {
	visibleLines := m.height - 4
	if visibleLines < 1 {
		visibleLines = 10
	}

	if m.cursor >= m.viewportTop+visibleLines {
		m.viewportTop = m.cursor - visibleLines + 1
	}

	if m.cursor < m.viewportTop {
		m.viewportTop = m.cursor
	}

	if m.viewportTop < 0 {
		m.viewportTop = 0
	}
}

// sortDirectoryContents returns sorted copies of files and subdirectories.
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
			result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		case SortByType:
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
	return ""
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
			ErrorCount:   len(errors),
			Errors:       errors,
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

func (m *Model) integrateDirectoryIntoTree(dirInfo *scanner.DirInfo) {
	parentPath := filepath.Dir(dirInfo.Path)
	log.Printf("DEBUG: Looking for parent directory: %s", parentPath)

	// Find the parent directory in the tree
	parentDir := m.findDirectoryInTree(m.rootDir, parentPath)
	if parentDir != nil {
		log.Printf("DEBUG: Found parent directory, updating subdirs")
		// Find the corresponding subdir entry and replace it with the loaded data
		for i, subdir := range parentDir.Subdirs {
			if subdir.Path == dirInfo.Path {
				log.Printf("DEBUG: Replacing subdir at index %d with loaded data", i)
				parentDir.Subdirs[i] = *dirInfo
				break
			}
		}
	} else {
		log.Printf("DEBUG: Parent directory not found in tree: %s", parentPath)
	}
}

// View renders the current state
func (m Model) View() string {
	return m.ViewTree()
}