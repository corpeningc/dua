package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
)


type Model struct {
	// Directory data
	rootDir *scanner.DirInfo
	currentPath string

	// UI state
	cursor int // Which item is selected
	expanded map[string]bool // Which directories are expanded

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

		width: 80,
		height: 24,
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
					}
				case "down", "j":
					m.cursor ++
				case "right", "l", "enter":
					// Expand directory
				case "left", "h":
					// Collapse directory
				}
		}
		return m, nil
	}

// View renders the current state
func (m Model) View() string {
	return m.ViewTree()
}