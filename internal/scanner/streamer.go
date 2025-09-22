package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type StreamingUpdate struct {
	Path string
	FileCount int
	DirCount int
	TotalSize int64
	DirInfo *DirInfo
	IsComplete bool
	ScanTime time.Duration
}

type StreamingScanner struct {
	maxWorkers int

	// Channels
	workQueue chan string
	updateChan chan StreamingUpdate
	errorChan chan error

	// Control
	context context.Context
	cancel context.CancelFunc
	workerGroup sync.WaitGroup
	

	// State tracking
	activeJobs int64
	jobMutex sync.Mutex
}

func NewStreamingScanner() *StreamingScanner {
	context, cancel := context.WithCancel(context.Background())

	return &StreamingScanner{
		maxWorkers: runtime.NumCPU() * 4,
		workQueue: make(chan string, 100),
		updateChan: make(chan StreamingUpdate, 50),
		errorChan: make(chan error, 10),
		context: context,
		cancel: cancel,
		activeJobs: 0,
	}
}

func (s *StreamingScanner) StartStreaming(rootPath string) (<-chan StreamingUpdate, <-chan error) {
	for i := 0; i < s.maxWorkers; i++ {
		s.workerGroup.Add(1)
		go s.worker(i)
	}

	go s.monitorCompletion()
	s.queueWork(rootPath)

	return s.updateChan, s.errorChan
}

func (s *StreamingScanner) worker(id int) {
	defer s.workerGroup.Done()
	for {
		select {
		case dirPath, ok := <-s.workQueue:
			if !ok {
				return
			}

			s.incrementActiveJobs()
			update := s.scanDirectory(dirPath, id)
			s.decrementActiveJobs()

			if update != nil {
				select {
				case s.updateChan <- *update:
				case <-s.context.Done():
					return
				}

				for _, subdir := range update.DirInfo.Subdirs {
					log.Printf("DEBUG: About to queue: %s", subdir.Path)
					s.queueWork(subdir.Path)
				}
			}
		case <-s.context.Done():
			return
		}
	}
}

func (s *StreamingScanner) Stop() {
	s.cancel()
	s.workerGroup.Wait()

	close(s.workQueue)
	close(s.updateChan)
	close(s.errorChan)
}

func (s *StreamingScanner) scanDirectory(path string, workerID int) *StreamingUpdate {
	startTime := time.Now()

	entries, err := os.ReadDir(path)

	if err != nil {
		select {
			case s.errorChan <- fmt.Errorf("Error reading directory %s: %v\n", path, err):
			case <-s.context.Done():
		}
		return nil
	}

	dirInfo := DirInfo{
		Path: path,
		Size: 0,
		Files: []FileInfo{},
		Subdirs: []DirInfo{},
		IsLoaded: true,
		IsLoading: false,
	}

	var fileCount, dirCount, totalBytes int64

	for _, entry := range entries {
		select {
		case <-s.context.Done():
			return nil
		default:
		}

		if entry.IsDir() {
			fullPath := filepath.Join(path, entry.Name())
			subdir := DirInfo {
				Path: fullPath,
				Size: 0,
				Files: []FileInfo{},
				Subdirs: []DirInfo{},
				IsLoaded: false,
				IsLoading: false,
				FileCount: 0,
				SubdirCount: 0,
			}

			dirInfo.Subdirs = append(dirInfo.Subdirs, subdir)
			dirCount++
		} else {
			if info, err := entry.Info(); err == nil {
				file := FileInfo {
					Name: entry.Name(),
					Size: info.Size(),
				}

				dirInfo.Files = append(dirInfo.Files, file)
				fileCount++
				totalBytes += info.Size()
			}
		}
	}

	dirInfo.Size = totalBytes
	dirInfo.FileCount = int(fileCount)
	dirInfo.SubdirCount = int(dirCount)

	scanDuration := time.Since(startTime)

	return &StreamingUpdate{
		Path: path,
		FileCount: int(fileCount),
		DirCount: int(dirCount),
		TotalSize: totalBytes,
		DirInfo: &dirInfo,
		IsComplete: false,
		ScanTime: scanDuration,
	}
}

func (s *StreamingScanner) queueWork(path string) {
	select {
	case s.workQueue <- path:
	case <-s.context.Done():
	}
}

func (s *StreamingScanner) incrementActiveJobs() {
	s.jobMutex.Lock()
	s.activeJobs++
	s.jobMutex.Unlock()
}

func (s *StreamingScanner) decrementActiveJobs() {
	s.jobMutex.Lock()
	s.activeJobs--
	s.jobMutex.Unlock()
}

func (s *StreamingScanner) getActiveJobs() int64 {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()
	return s.activeJobs
}

func (s *StreamingScanner) monitorCompletion() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if len(s.workQueue) == 0 && s.getActiveJobs() == 0 {
				// Wait a bit more to be sure
				time.Sleep(100 * time.Millisecond)
				if len(s.workQueue) == 0 && s.getActiveJobs() == 0 {
					select {
					case s.updateChan <- StreamingUpdate{IsComplete: true}:
					case <-s.context.Done():
					}
					return
				}
		}
		case <-s.context.Done():
			return
		}
	}
}