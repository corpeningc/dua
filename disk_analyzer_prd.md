# Disk Usage Analyzer - Product Requirements Document

## Overview

A fast, interactive terminal-based disk usage analyzer that helps developers and system administrators identify large files and directories to reclaim disk space efficiently.

## Problem Statement

- Current disk usage tools (like `du`, `ncdu`, WinDirStat) are either too slow, lack interactivity, or don't provide efficient navigation for large directory trees
- Developers need to quickly identify space-consuming files in complex project structures (node_modules, build artifacts, logs)
- Existing tools don't offer modern UX patterns like fuzzy search, bookmarks, or intelligent cleanup suggestions

## Target Users

**Primary**: Software developers managing local development environments
**Secondary**: System administrators maintaining servers and workstations

## Core Features

### 1. Interactive Terminal UI (TUI)
- **Tree-style directory navigation** with expand/collapse functionality
- **Real-time size display** for all files and directories
- **Responsive interface** that works on various terminal sizes
- **Vim-style keyboard navigation** (hjkl) with arrow key alternatives

### 2. Performance & Scalability
- **Lazy loading** - scan directories on-demand as user navigates
- **Background scanning** - continue scanning deeper levels while user browses
- **Caching system** - remember scanned results for faster re-navigation
- **Pagination** - handle directories with thousands of files efficiently
- **Progress indicators** for long-running scans

### 3. Navigation & Search
- **Fuzzy search** - instantly filter current view by typing
- **Breadcrumb navigation** - always show current path
- **Jump to parent** - quick navigation up multiple directory levels
- **Bookmarks** - save and quickly access frequently used deep paths
- **Sort modes** - by size, name, modification date, file count

### 4. Analysis & Insights
- **File type breakdown** - categorize space usage by extension (.log, .tmp, .js, etc.)
- **Size thresholds** - highlight files/directories above configurable sizes
- **Duplicate detection** - identify potentially duplicate large files
- **Age analysis** - highlight old files that might be safe to remove

## Nice-to-Have Features

### 1. Advanced Analysis
- **Cleanup suggestions** - identify temp files, caches, old logs based on patterns
- **Storage trends** - track changes over time if run repeatedly on same directory
- **Cleanup profiles** - predefined rules for common scenarios (dev environments, system cleanup)
- **Network drive support** - optional flag to include network-mounted directories in scans

### 2. Reporting & Export
- **Export reports** - JSON/CSV format for further analysis or scripting
- **Comparison mode** - see changes between different scan sessions
- **Summary statistics** - total size, file counts, largest items
- **Historical tracking** - track disk usage trends over time

### 3. Configuration & System Integration
- **Settings persistence** - store user preferences in profile directory (`~/.config/diskanalyzer/` on Unix, `%APPDATA%\diskanalyzer\` on Windows)
- **Ignore patterns** - exclude directories like node_modules, .git by default
- **Custom color schemes** - adapt to different terminal preferences
- **Configurable hotkeys** - allow users to customize navigation keys
- **Network mount handling** - skip network drives by default, enable with `--include-network` flag
- **Permission handling** - display sizes for inaccessible directories with "(read-only)" indicator

## Technical Requirements

### Performance Targets
- **Scan speed**: Handle 10,000+ files in under 2 seconds
- **Memory usage**: Stay under 100MB for typical directory structures
- **UI responsiveness**: Maintain 60fps navigation even during background scanning
- **Startup time**: Launch in under 500ms

### Compatibility
- **Operating Systems**: Linux, macOS, Windows
- **Terminal compatibility**: Works with most modern terminal emulators
- **Go version**: Compatible with Go 1.19+
- **Architecture**: Support x86_64, ARM64

### Dependencies
- **bubbletea** - TUI framework and event handling
- **lipgloss** - Styling and layout
- **bubbles** - Pre-built UI components (progress bars, lists)
- **golang.org/x/sys** - Cross-platform file system operations

## User Experience

### Initial Launch
```
$ diskanalyzer
Scanning current directory...
[████████████████████████████████████] 100%

/home/user/projects                           2.3 GB
├── 📁 node_modules/                         1.8 GB
├── 📁 build/                                320 MB
├── 📁 .git/                                 156 MB
├── 📁 src/                                   45 MB
└── 📄 package-lock.json                       2 MB

Navigation: ↑↓ to move, → to expand, ← to collapse, / to search, q to quit
```

### Navigation Flow
1. **Browse**: Use arrow keys or vim keys to navigate tree
2. **Expand**: Enter or → to dive into directories
3. **Search**: Press `/` to fuzzy search current level
4. **Sort**: Press `s` to cycle through sort modes
5. **Analyze**: View file type breakdown with `t`
6. **Bookmark**: Press `b` to bookmark current location

### Key Bindings
- **Navigation**: `↑↓hjkl` - move cursor, `→l/Enter` - expand, `←h/Backspace` - collapse
- **Search**: `/` - fuzzy search, `Esc` - clear search
- **Sorting**: `s` - cycle sort modes, `r` - reverse order
- **Actions**: `Space` - toggle selection, `Del` - delete prompt, `b` - bookmark
- **Views**: `t` - file type breakdown, `i` - item details, `?` - help
- **System**: `q` - quit, `Ctrl+C` - force quit, `Ctrl+R` - refresh

## Success Metrics

### Usability
- Time to identify largest directory < 30 seconds
- User can navigate to any location within 10 keystrokes
- Zero crashes during normal operation

### Performance
- Handles repositories with 50,000+ files smoothly
- Background scanning doesn't block UI interaction
- Memory usage scales predictably with directory size

## Implementation Phases

### Phase 1: MVP (Week 1-2)
- Basic tree navigation with size display
- Directory expansion/collapse
- Simple keyboard navigation
- Size-based sorting

### Phase 2: Core Features (Week 3-4)
- Fuzzy search functionality
- Background scanning with progress bars
- Multiple sort modes
- Basic caching system
- File deletion with trash integration

### Phase 3: Polish (Week 5-6)
- File type analysis
- Permission handling for restricted directories
- Bookmarks and advanced navigation
- Configuration system with profile directory storage

### Phase 4: Advanced (Future)
- Export functionality
- Historical tracking
- Advanced cleanup automation
- Plugin system

## Implementation Details

### File Management Specifications
- **Large file threshold**: Files/directories over 1GB require double confirmation before deletion
- **Deletion queue limit**: Maximum 100 queued deletions to prevent memory issues
- **Concurrent deletions**: Process up to 3 files simultaneously for optimal speed/system balance
- **Error log retention**: Keep last 1000 error entries in memory, rotate automatically

### Performance Optimizations
- **Navigation priority**: All UI interactions prioritized over background operations
- **Memory efficiency**: Lazy loading with aggressive cleanup of unused tree branches
- **Responsive feedback**: Progress indicators for any operation taking >500ms

## Success Definition

The project is successful if:
- Developers can quickly identify space-wasting files in their projects
- The tool performs significantly better than alternatives on large directory structures
- Users find the interface intuitive and efficient
- The open source project gains community adoption and contributions