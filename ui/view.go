package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/corpeningc/dua/internal/scanner"
)

var (
	selectedStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#5C5C5C"))

	directoryStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#04B575"))

	fileStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF"))

	sizeStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#626262")).
	Align(lipgloss.Right)
)


func (m Model) ViewTree() string {
	var b strings.Builder

	// Header
	header := fmt.Sprintf("DUA - Disk Usage Analyzer | Path: %s", m.currentPath)
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("-", len(header)) + "\n")

	if m.rootDir != nil {
		m.renderDirectory(&b, m.rootDir, 0, 0)
	}

	// Footer with controls
	b.WriteString("\n")
	controls := "↑↓/jk: navigate • →l: expand • ←h: collapse • q: quit"
	b.WriteString(controls + "\n")

	return b.String()
}

func (m Model) renderDirectory(b *strings.Builder, dir *scanner.DirInfo, depth int, currentIndex int) int {
	indent := strings.Repeat("  ", depth)

	// Dir name and size
	dirName := fmt.Sprintf("📁 %s/", getBaseName(dir.Path))
	size := formatSize(dir.Size)

	line := fmt.Sprintf("%s%s", indent, dirName)

	// Highlight selected
	if currentIndex == m.cursor {
		line = selectedStyle.Render(line)
	} else {
		line = directoryStyle.Render(line)
	}

	// Add size
	line = fmt.Sprintf("%-50s %s", line, sizeStyle.Render(size))
	b.WriteString(line + "\n")

	currentIndex ++

	if depth == 0 || m.expanded[dir.Path] {
		// Show files
		for _, file := range dir.Files {
			fileIndent := strings.Repeat("  ", depth + 1)
			fileName := fmt.Sprintf("📄 %s", file.Name)
			fileSize := formatSize(file.Size)

			fileLine := fmt.Sprintf("%s%s", fileIndent, fileName)
			if currentIndex == m.cursor {
				fileLine = selectedStyle.Render(fileLine)
			} else {
				fileLine = fileStyle.Render(fileLine)
			}

			fileLine = fmt.Sprintf("%-50s %s", fileLine, sizeStyle.Render(fileSize))
			b.WriteString(fileLine + "\n")
			currentIndex++
		}

		for _, subdir := range dir.Subdirs {
			currentIndex = m.renderDirectory(b, &subdir, depth + 1, currentIndex)
		}
	}

	return currentIndex
}

// Helper funcs
func getBaseName(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}

	if len(parts) > 1 {
		return parts[len(parts)-2]
	}

	return path
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n:= bytes / div; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}