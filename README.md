# DUA - Disk Usage Analyzer

A fast, interactive terminal-based directory analysis tool built in Go using the Charm TUI framework. DUA provides an intuitive tree view for exploring directory sizes with lazy loading capabilities and efficient parallel scanning.

## Features

- **Interactive Tree View**: Navigate through directories with expand/collapse functionality
- **Lazy Loading**: Fast initial scanning with on-demand directory content loading
- **Parallel Scanning**: Efficient multi-threaded directory traversal
- **Multiple Sorting Options**: Sort by name, size, date, or type (ascending/descending)
- **Memory Efficient**: Semaphore-controlled goroutines prevent resource exhaustion
- **Cross-Platform**: Works on Windows, macOS, and Linux

## Installation

### Prerequisites
- Go 1.19 or later

### Quick Install

Install directly from GitHub:
```bash
go install github.com/corpeningc/dua/cmd/dua@latest
```

Then run:
```bash
dua (runs in current directory)
dua --path {path}
```

### Build from Source (Alternative)

1. Clone the repository:
```bash
git clone https://github.com/corpeningc/dua.git
cd dua
```

2. Build the executable:
```bash
go build -o dua ./cmd/dua
```

## Usage

### Basic Usage

Run DUA on the current directory:
```bash
dua
```

Analyze a specific directory:
```bash
dua -path /path/to/analyze
```

### Development

Run directly with Go:
```bash
go run ./cmd/dua
```

Run with specific path:
```bash
go run ./cmd/dua -path C:\Users\Username
```

### Keyboard Controls

- **↑/↓ or j/k**: Navigate up/down
- **Enter or →**: Expand directory / Enter subdirectory
- **← or Backspace**: Collapse directory / Go to parent
- **s**: Toggle sort mode (Name → Size → Date → Type)
- **r**: Reverse sort order
- **q**: Quit

## Use Cases

### System Administration
- **Disk Space Analysis**: Quickly identify which directories consume the most space
- **Cleanup Planning**: Find large files and directories for cleanup operations

## Architecture

DUA uses a lazy loading architecture with two scanning phases:

1. **Initial Scan**: Fast parallel calculation of directory sizes only
2. **Lazy Loading**: Directory contents loaded on-demand when expanded

This approach provides immediate feedback while maintaining responsiveness for large directory trees.

### Key Components

- **Scanner**: Handles directory traversal with semaphore-controlled concurrency
- **UI**: Bubble Tea-based terminal interface with tree navigation
- **Main**: CLI entry point with command-line argument parsing

## Performance

DUA is optimized for performance through:

- **Concurrent Scanning**: Uses `runtime.NumCPU() * 2` goroutines for parallel processing
- **Memory Management**: Semaphore pattern prevents goroutine explosion
- **Lazy Loading**: Only loads directory contents when needed
- **Efficient Rendering**: Viewport-based rendering for large directory trees

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request
