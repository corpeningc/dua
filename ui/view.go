package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/corpeningc/dua/internal/scanner"
)

var ( 
	selectedItemStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#7D56F4")).  // Purple background      
	Foreground(lipgloss.Color("#FFFFFF"))   // White text

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

	markedForDeletionStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.Color("#CC0000"))
)


func (m Model) ViewTree() string {
	var b strings.Builder

	// Header
	direction := "↓"
	if m.sortAsc {
		direction = "↑"
	}

	header := fmt.Sprintf("DUA - Disk Usage Analyzer | Path: %s | Sort: %s%s", m.currentPath, m.sortMode.String(), direction)
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("-", len(header)) + "\n")

	var contentBuilder strings.Builder
	if m.rootDir != nil {
		visibleLines := m.height - 4 // Reserve space for header and footer
		if visibleLines < 1 {
			visibleLines = 10
		}
		m.renderDirectoryWithViewport(&contentBuilder, m.rootDir, 0, 0, m.viewportTop, visibleLines)
	}

	b.WriteString(contentBuilder.String())

	// Footer with controls
	b.WriteString("\n")
	var controls string
	if m.deletionMode {
		controls = fmt.Sprintf("%d marked for deletion • d: DELETE • esc: cancel", len(m.markedForDeletion))
	}	else {
		controls = "↑↓/jk: navigate • →l: expand • ←h: collapse • s: sort • ctrl+s: reverse sort • q: quit"
	}
	b.WriteString(controls + "\n")

	return b.String()
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

func (m Model) countVisibleItems() int {
	if m.rootDir == nil{
		return 0
	}

	return m.countDirectoryItems(m.rootDir, 0)
}


func (m Model) getCurrentItem() (string, bool) {
	if m.rootDir == nil {
		return "", false
	}

	return m.findItemAtIndex(m.rootDir, 0, 0, m.cursor)
}

func (m Model) findItemAtIndex(dir *scanner.DirInfo, depth int, currentIndex int, targetIndex int) (string, bool) {
	if currentIndex == targetIndex {
		return dir.Path, true
	}

	currentIndex++

	// If expanded, check contents
	if depth == 0 || m.expanded[dir.Path] {
		sortedFiles, sortedSubdirs := m.sortDirectoryContents(dir)
		for _, file := range sortedFiles {
			if currentIndex == targetIndex {
				return filepath.Join(dir.Path, file.Name), false
			}
			currentIndex++
		}

		for _, subdir := range sortedSubdirs {
			if path, isDir := m.findItemAtIndex(&subdir, depth + 1, currentIndex, targetIndex); path != "" {
				return path, isDir
			}
			
			currentIndex += m.countDirectoryItems(&subdir, depth + 1)
		}
	}

	return "", false
}

func (m Model) countDirectoryItems(dir *scanner.DirInfo, depth int) int {
	// Count current directory
	count := 1

	if depth == 0 || m.expanded[dir.Path] {
		count += len(dir.Files)

		for _, subdir := range dir.Subdirs {
			count += m.countDirectoryItems(&subdir, depth+1)
		}
	}
	
	return count
}


func (m Model) renderDirectoryWithViewport(b *strings.Builder, dir *scanner.DirInfo, depth int, currentIndex int, viewportTop int, maxLines int) int {
	// Check if we should render this directory
	linesUsed := strings.Count(b.String(), "\n")
	if linesUsed >= maxLines {
		return currentIndex
	}

	if currentIndex >= viewportTop {
		indent := strings.Repeat("  ", depth)
		dirName := fmt.Sprintf("📁 %s/", getBaseName(dir.Path))
		var size string
		if dir.IsLoading {
			size = "Loading..."
		} else {
			size = formatSize(dir.Size)
		}

		line := fmt.Sprintf("%s%s", indent, dirName)

		if currentIndex == m.cursor {
			line = selectedStyle.Render(line)
		} else if m.markedForDeletion[dir.Path] {
			line = markedForDeletionStyle.Render(line)
		} else if m.selected[dir.Path] {
			line = selectedItemStyle.Render(line)
		} else {
			line = directoryStyle.Render(line)
		}

		line = fmt.Sprintf("%-50s %s", line, sizeStyle.Render(size))
		b.WriteString(line + "\n")
	}
	currentIndex++

	// Render contents if expanded
	if (depth == 0 || m.expanded[dir.Path]) && linesUsed < maxLines{
		// Files
		sortedFiles, sortedSubdirs := m.sortDirectoryContents(dir)
		for _, file := range sortedFiles {
			linesUsed = strings.Count(b.String(), "\n")
			if linesUsed >= maxLines {
				break
			}

			if currentIndex >= viewportTop {
				fileIndent := strings.Repeat("  ", depth + 1)
				fileName := fmt.Sprintf("📄 %s", file.Name)
				fileSize := formatSize(file.Size)

				filePath := filepath.Join(dir.Path, file.Name)
				fileLine := fmt.Sprintf("%s%s", fileIndent, fileName)

				if currentIndex == m.cursor {
					fileLine = selectedStyle.Render(fileLine)
				} else if m.markedForDeletion[filePath] {
					fileLine = markedForDeletionStyle.Render(fileLine)
				} else if m.selected[filePath] {
					fileLine = selectedItemStyle.Render(fileLine)
				} else {
					fileLine = fileStyle.Render(fileLine)
				}

				fileLine = fmt.Sprintf("%-50s %s", fileLine, sizeStyle.Render(fileSize))
				b.WriteString(fileLine + "\n")
			}
			currentIndex++
		}

		// Subdirectories
		for _, subdir := range sortedSubdirs {
			linesUsed = strings.Count(b.String(), "\n")
			if linesUsed >= maxLines {
				break
			}
			currentIndex = m.renderDirectoryWithViewport(b, &subdir, depth+1, currentIndex, viewportTop, maxLines)
		}
	}

	return currentIndex
}