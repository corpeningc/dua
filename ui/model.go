package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
)

type LoadingCompleteMsg struct {
	Path string
	Success bool
	Error error
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

			case LoadingCompleteMsg:
				dirInfo := m.findDirectoryInTree(m.rootDir, msg.Path)
				if dirInfo != nil {
					dirInfo.IsLoading = false
					if msg.Success {
						dirInfo.IsLoaded = true
					}
				}

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

// View renders the current state
func (m Model) View() string {
	return m.ViewTree()
}