package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
)

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

	// UI state
	cursor int // Which item is selected
	expanded map[string]bool // Which directories are expanded
	viewportTop int // First visible item index

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
		viewportTop: 0,

		width: 80,
		height: 24,

		sortMode: SortByName,
		sortAsc: false,
	}
}

	func (m Model) Init() tea.Cmd {
		return nil
	}

	func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
		switch msg := msg.(type) {
			case tea.WindowSizeMsg:
				m.width = msg.Width
				m.height = msg.Height

			case tea.KeyMsg:
				switch msg.String() {
				case "ctrl+c", "q":
					return m, tea.Quit
				case "up", "k":
					if m.cursor > 0 {
						m.cursor--
						m.adjustViewport()
					}
				case "down", "j":
					maxItems := m.countVisibleItems()
					if m.cursor < maxItems - 1 {
						m.cursor++
						m.adjustViewport()
					}
				case "right", "l", "enter":
					// Expand directory with lazy loading
					if path, isDir := m.getCurrentItem(); isDir && path != "" {
						m.expanded[path] = true
						// Trigger lazy loading if not already loaded
						m.loadDirectoryContents(path)
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

// loadDirectoryContents triggers lazy loading for a directory path
func (m *Model) loadDirectoryContents(path string) {
	// Find the directory in our tree and load its contents
	m.loadDirectoryInTree(m.rootDir, path)
}

// loadDirectoryInTree recursively finds and loads a directory
func (m *Model) loadDirectoryInTree(dir *scanner.DirInfo, targetPath string) {
	if dir.Path == targetPath && !dir.IsLoaded && !dir.IsLoading {
		// Load this directory's contents
		scanner.LoadDirectoryContents(dir)
		return
	}

	// Search in subdirectories
	for i := range dir.Subdirs {
		m.loadDirectoryInTree(&dir.Subdirs[i], targetPath)
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

// View renders the current state
func (m Model) View() string {
	return m.ViewTree()
}