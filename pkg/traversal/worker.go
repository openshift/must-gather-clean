package traversal

import (
	"fmt"

	"github.com/openshift/must-gather-clean/pkg/cleaner"
	"k8s.io/klog/v2"
)

type fileProcessingError struct {
	path  string
	cause error
}

func (f *fileProcessingError) Error() string {
	return fmt.Sprintf("failed to process %s: %v", f.path, f.cause)
}

func (f *fileProcessingError) Cause() error {
	return f.cause
}

// workerInput here is a relative path to the must-gather root folder
type workerInput string

type QueueProcessor interface {
	ProcessQueue(queue chan workerInput, errorCh chan error)
}

type Worker struct {
	id      int
	cleaner cleaner.Processor
}

func (w *Worker) ProcessQueue(queue chan workerInput, errorCh chan error) {
	for wf := range queue {
		path := string(wf)
		klog.V(3).Infof("[Worker %02d] Processing %s\n", w.id, path)

		err := w.cleaner.Process(path)
		if err != nil {
			errorCh <- &fileProcessingError{
				path:  path,
				cause: err,
			}
		}

		klog.V(3).Infof("[Worker %02d] Finished processing %s\n", w.id, path)
	}
}

func NewWorker(id int, cleaner cleaner.Processor) QueueProcessor {
	return &Worker{
		id:      id,
		cleaner: cleaner,
	}
}
