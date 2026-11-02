package traversal

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type collectingQueueProcessor struct {
	paths []string
}

func (c *collectingQueueProcessor) ProcessQueue(queue chan workerInput, _ chan error) {
	for wf := range queue {
		c.paths = append(c.paths, string(wf))
	}
}

func TestFileWalker(t *testing.T) {
	for _, tc := range []struct {
		name           string
		inputDir       string
		expectedResult []string
	}{
		{
			name:     "basic",
			inputDir: "testfiles/test1/mg",
			expectedResult: []string{
				"nodes/another.yaml",
				"nodes/test.yaml",
				"pods/pod1/application.log",
				"pods/pod1/manifests.yaml",
				"pods/pod2/application.log",
				"pods/pod2/manifests.yaml",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queueProc := &collectingQueueProcessor{[]string{}}
			walker := NewParallelFileWalker(tc.inputDir, 1, func(id int) QueueProcessor {
				return queueProc
			})

			require.NoError(t, walker.Traverse())

			assert.Equal(t, tc.expectedResult, queueProc.paths)
		})
	}
}

func TestFileWalkerAbsolutePathing(t *testing.T) {
	abs, err := filepath.Abs("testfiles/test1/mg")
	require.NoError(t, err)

	queueProc := &collectingQueueProcessor{[]string{}}
	walker := NewParallelFileWalker(abs, 1, func(id int) QueueProcessor {
		return queueProc
	})

	require.NoError(t, walker.Traverse())

	assert.Equal(t, []string{
		"nodes/another.yaml",
		"nodes/test.yaml",
		"pods/pod1/application.log",
		"pods/pod1/manifests.yaml",
		"pods/pod2/application.log",
		"pods/pod2/manifests.yaml",
	}, queueProc.paths)
}

func TestFileWalkerReturnsWorkerError(t *testing.T) {
	processor := &failingQueueProcessor{}
	walker := NewParallelFileWalker("testfiles/test1/mg", 1, func(id int) QueueProcessor {
		return processor
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorContains(t, err, "worker failed")
	assert.NotEmpty(t, processor.paths)
	assert.Less(t, len(processor.paths), 6, "worker error should stop the walk before every file is queued")
}

func TestFileWalkerJoinsWorkerErrors(t *testing.T) {
	firstErr := errors.New("first worker failed")
	secondErr := errors.New("second worker failed")
	ready := sync.WaitGroup{}
	ready.Add(2)

	walker := NewParallelFileWalker("testfiles/test1/mg", 2, func(id int) QueueProcessor {
		err := firstErr
		if id == 2 {
			err = secondErr
		}
		return &synchronizedFailingQueueProcessor{ready: &ready, err: err}
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorIs(t, err, firstErr)
	assert.ErrorIs(t, err, secondErr)
}

type failingQueueProcessor struct {
	paths []string
}

func (f *failingQueueProcessor) ProcessQueue(queue chan workerInput, errorCh chan error) {
	for input := range queue {
		f.paths = append(f.paths, string(input))
		errorCh <- errors.New("worker failed")
	}
}

type synchronizedFailingQueueProcessor struct {
	ready *sync.WaitGroup
	err   error
}

func (p *synchronizedFailingQueueProcessor) ProcessQueue(queue chan workerInput, errorCh chan error) {
	if _, ok := <-queue; !ok {
		return
	}
	p.ready.Done()
	p.ready.Wait()
	errorCh <- p.err
	for range queue {
	}
}
