package traversal

import (
	"errors"
	"fmt"
	"os"
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
	workerErr := errors.New("worker failed")
	processor := &failingQueueProcessor{err: workerErr}
	walker := NewParallelFileWalker("testfiles/test1/mg", 1, func(id int) QueueProcessor {
		return processor
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorIs(t, err, workerErr)
	assert.NotEmpty(t, processor.paths)
	assert.Less(t, len(processor.paths), 6, "worker error should stop the walk before every file is queued")
}

func TestFileWalkerJoinsWorkerErrors(t *testing.T) {
	inputDir := t.TempDir()
	for i := 1; i <= 20; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(inputDir, fmt.Sprintf("%02d.txt", i)), nil, 0o600))
	}

	firstErr := errors.New("first worker failed")
	secondErr := errors.New("second worker failed")
	firstCleanupErr := errors.New("first worker cleanup failed")
	secondCleanupErr := errors.New("second worker cleanup failed")
	firstFinalErr := errors.New("first worker final failure")
	secondFinalErr := errors.New("second worker final failure")
	ready := sync.WaitGroup{}
	ready.Add(2)

	walker := NewParallelFileWalker(inputDir, 2, func(id int) QueueProcessor {
		errs := []error{firstErr, firstCleanupErr, firstFinalErr}
		if id == 2 {
			errs = []error{secondErr, secondCleanupErr, secondFinalErr}
		}
		return &synchronizedFailingQueueProcessor{ready: &ready, errs: errs}
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorIs(t, err, firstErr)
	assert.ErrorIs(t, err, secondErr)
	assert.ErrorIs(t, err, firstCleanupErr)
	assert.ErrorIs(t, err, secondCleanupErr)
	assert.ErrorIs(t, err, firstFinalErr)
	assert.ErrorIs(t, err, secondFinalErr)
}

func TestFileWalkerJoinsMultipleErrorsFromOneWorker(t *testing.T) {
	inputDir := t.TempDir()
	for i := 1; i <= 4; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(inputDir, fmt.Sprintf("%02d.txt", i)), nil, 0o600))
	}

	firstErr := errors.New("first file failed")
	secondErr := errors.New("second file failed")
	walker := NewParallelFileWalker(inputDir, 1, func(id int) QueueProcessor {
		return &twoErrorQueueProcessor{firstErr: firstErr, secondErr: secondErr}
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorIs(t, err, firstErr)
	assert.ErrorIs(t, err, secondErr)
}

func TestFileWalkerReturnsTraversalError(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "missing")
	walker := NewParallelFileWalker(missingDir, 1, func(id int) QueueProcessor {
		return &collectingQueueProcessor{}
	})

	err := walker.Traverse()
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

type failingQueueProcessor struct {
	paths []string
	err   error
}

func (f *failingQueueProcessor) ProcessQueue(queue chan workerInput, errorCh chan error) {
	for input := range queue {
		f.paths = append(f.paths, string(input))
		errorCh <- f.err
	}
}

type synchronizedFailingQueueProcessor struct {
	ready *sync.WaitGroup
	errs  []error
}

func (p *synchronizedFailingQueueProcessor) ProcessQueue(queue chan workerInput, errorCh chan error) {
	if _, ok := <-queue; !ok {
		return
	}
	p.ready.Done()
	p.ready.Wait()
	for _, err := range p.errs {
		errorCh <- err
	}
	for range queue {
	}
}

type twoErrorQueueProcessor struct {
	firstErr  error
	secondErr error
}

func (p *twoErrorQueueProcessor) ProcessQueue(queue chan workerInput, errorCh chan error) {
	if _, ok := <-queue; !ok {
		return
	}
	if _, ok := <-queue; !ok {
		return
	}
	errorCh <- p.firstErr
	errorCh <- p.secondErr
	for range queue {
	}
}
