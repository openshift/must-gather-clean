package traversal

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
)

type Traverser interface {
	Traverse() error
}

type FileWalker struct {
	inputPath     string
	workerCount   int
	workers       []QueueProcessor
	workerFactory func(int) QueueProcessor
}

// Traverse processes the must-gather directory and returns the first worker
// or traversal error. It lets the caller decide how to report and handle the
// failure instead of terminating the process from this package.
func (w *FileWalker) Traverse() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	errorCh := make(chan error, w.workerCount)
	queue := make(chan workerInput, w.workerCount)
	w.workers = make([]QueueProcessor, w.workerCount)
	for i := 0; i < w.workerCount; i++ {
		w.workers[i] = w.workerFactory(i + 1)
		wg.Add(1)
		go func(i int, queue chan workerInput, errorCh chan error) {
			w.workers[i].ProcessQueue(queue, errorCh)
			wg.Done()
		}(i, queue, errorCh)
	}

	firstError := make(chan error, 1)
	collectionDone := make(chan struct{})
	go func(errorCh <-chan error) {
		defer close(collectionDone)
		for err := range errorCh {
			select {
			case firstError <- err:
			default:
			}
			cancel()
		}
	}(errorCh)

	walkErr := filepath.WalkDir(w.inputPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return fs.SkipAll
		default:
		}

		if !dirEntry.IsDir() {
			// the rest of the logic expects the path to be relative to the input dir root, if it fails we assume it is already relative
			relPath, err := filepath.Rel(w.inputPath, path)
			if err != nil {
				queue <- workerInput(path)
			} else {
				queue <- workerInput(relPath)
			}
		}

		return nil
	})

	close(queue)
	wg.Wait()

	// Once all workers have exited, close the error channel and wait for the
	// collector so a buffered worker error cannot be missed.
	close(errorCh)
	<-collectionDone

	select {
	case err := <-firstError:
		return err
	default:
	}
	return walkErr
}

func NewParallelFileWalker(inputPath string, workerCount int, workerFactory func(id int) QueueProcessor) *FileWalker {
	return &FileWalker{
		inputPath:     inputPath,
		workerCount:   workerCount,
		workerFactory: workerFactory,
	}
}
