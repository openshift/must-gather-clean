package traversal

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/schollz/progressbar/v3"
	"k8s.io/klog/v2"
)

type Traverser interface {
	Traverse()
}

type FileWalker struct {
	inputPath     string
	workerCount   int
	workers       []QueueProcessor
	workerFactory func(int) QueueProcessor
}

// Traverse should be called to start processing the must-gather directory. This method will exit the CLI if an error is encountered.
func (w *FileWalker) Traverse() {
	wg := sync.WaitGroup{}
	errorCh := make(chan error, w.workerCount)
	queue := make(chan workerInput, w.workerCount)
	w.workers = make([]QueueProcessor, w.workerCount)
	progressBar := progressbar.NewOptions(
		w.workerCount,
		progressbar.OptionSetDescription("Waiting for workers"),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionShowCount(),
		progressbar.OptionFullWidth(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
	for i := 0; i < w.workerCount; i++ {
		w.workers[i] = w.workerFactory(i + 1)
		wg.Add(1)
		go func(i int, queue chan workerInput, errorCh chan error) {
			w.workers[i].ProcessQueue(queue, errorCh)
			progressBar.Add(1)
			wg.Done()
		}(i, queue, errorCh)
	}

	errorWg := sync.WaitGroup{}
	errorWg.Add(1)
	go func(errorCh <-chan error) {
		for err := range errorCh {
			var e *fileProcessingError
			switch {
			case errors.As(err, &e):
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
				queue <- workerInput(path)
			} else {
				queue <- workerInput(relPath)
			}
		}

		return nil
	})

	if err != nil {
		klog.Exitf("failed to traverse the directory structure due to: %v", err)
	}

	close(queue)
	wg.Wait()
	// need to add an empty line when the bar finishes with non default options
	progressBar.Finish()
	fmt.Println()

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
