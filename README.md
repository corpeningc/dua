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
go install github.com/corpeningc/dua@latest
```

Then run:
```bash
dua
```
or

``` bash
dua --path {path}
```
