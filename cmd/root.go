package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/corpeningc/dua/ui"
)

func Execute() error {
	// Set up debug logging
	logFile, err := os.Create("/tmp/dua-debug.log")
	if err == nil {
		log.SetOutput(logFile)
		log.Printf("=== DUA Debug Session Started ===")
		defer logFile.Close()
	}

	// Define command line flags
	var path string

	flag.StringVar(&path, "path", ".", "Directory path to analyze")
	flag.Parse()

	// Path validation
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: Path '%s' does not exist\n", path)
		os.Exit(1)
	}

	var model ui.Model

	fmt.Printf("Starting DUA for: %s\n", path)
	model = ui.NewStreamingModel(path)

	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	return nil
}
