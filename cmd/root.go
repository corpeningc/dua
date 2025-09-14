package cmd

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/internal/scanner"
	"github.com/corpeningc/dua/ui"
)

func Execute() error {
	// Define command line flags
	var path string
	var useStreaming bool

	flag.StringVar(&path, "path", ".", "Directory path to analyze")
	flag.BoolVar(&useStreaming, "streaming", true, "Use streaming mode for better performance (default: true)")
	flag.Parse()

	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: Path '%s' does not exist\n", path)
		os.Exit(1)
	}

	var model ui.Model

	if useStreaming {
		// Use new streaming mode - instant startup!
		fmt.Printf("Starting DUA in streaming mode for: %s\n", path)
		model = ui.NewStreamingModel(path)
	} else {
		// Fallback to legacy mode
		fmt.Printf("Scanning directory: %s\n", path)
		fmt.Printf("Please wait...\n")

		// Scan directory structure with lazy loading
		dirInfo, err := scanner.ScanDirectoryLazy(path)
		if err != nil {
			fmt.Printf("Error scanning directory: %v\n", err)
			os.Exit(1)
		}

		model = ui.NewModel(dirInfo, path)
	}

	// Start the TUI program
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program and handle any errors
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	return nil
}