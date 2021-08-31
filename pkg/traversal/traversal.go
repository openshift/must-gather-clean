package traversal

import (
	"io/fs"
	"path/filepath"
	"sync"

	"k8s.io/klog/v2"
)

type FileWalker struct {
	inputPath     string
	workerCount   int
	workers       []QueueProcessor
	workerFactory func(int) QueueProcessor
}

// Traverse should be called to start processing the must-gather directory. This method will exist the CLI if an error is encountered.
func (w *FileWalker) Traverse() {
	wg := sync.WaitGroup{}
	errorCh := make(chan error, w.workerCount)
	queue := make(chan WorkerInput, w.workerCount)
	w.workers = make([]QueueProcessor, w.workerCount)
	for i := 0; i < w.workerCount; i++ {
		w.workers[i] = w.workerFactory(i + 1)
		wg.Add(1)
		go func(i int, queue chan WorkerInput, errorCh chan error) {
			w.workers[i].ProcessQueue(queue, errorCh)
			wg.Done()
		}(i, queue, errorCh)
	}

	errorWg := sync.WaitGroup{}
	errorWg.Add(1)
	go func(errorCh <-chan error) {
		for err := range errorCh {
			switch e := err.(type) {
			case *fileProcessingError:
				klog.Exitf("failed to process %s due to %v", e.path, e.cause)
			default:
				klog.Exitf("unexpected error: %v", err)
			}

		}
		errorWg.Done()
	}(errorCh)

	err := filepath.WalkDir(w.inputPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dirEntry.IsDir() {
			// the rest of the logic expects the path to be relative to the input dir root, if it fails we assume it is already relative
			relPath, err := filepath.Rel(w.inputPath, path)
			if err != nil {
				queue <- WorkerInput{
					path: path,
				}
			} else {
				queue <- WorkerInput{
					path: relPath,
				}
			}
		}

		return nil
	})

	if err != nil {
		klog.Exitf("failed to traverse the directory structure due to: %v", err)
	}

	close(queue)
	wg.Wait()

	// once all the workers have exited close the error channel and wait for the exit goroutine to complete.
	close(errorCh)
	errorWg.Wait()
}

func NewParallelFileWalker(inputPath string, workerCount int, workerFactory func(id int) QueueProcessor) *FileWalker {
	return &FileWalker{
		inputPath:     inputPath,
		workerCount:   workerCount,
		workerFactory: workerFactory,
	}
}
