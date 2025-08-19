package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/corpeningc/dua/internal/scanner"
)

func main() {
	// Define command line flags
	var path string

	flag.StringVar(&path, "path", ".", "Directory path to analyze")
	flag.Parse()

	fmt.Printf("Disk Usage Analyzer\n")
	fmt.Printf("Analyzing path: %s\n", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: Path does '%s' not exist\n", path)
		os.Exit(1)
	}

	dirInfo, err := scanner.ScanDirectory(path)

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Size: %d bytes\n", dirInfo.Size)
	fmt.Printf("Found %d files and %d subdirectories\n", len(dirInfo.Files), len(dirInfo.Subdirs))

}