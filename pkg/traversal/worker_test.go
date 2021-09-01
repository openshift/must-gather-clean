package traversal

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type noOpCleaner struct {
	desiredError *error
}

func (n noOpCleaner) ProcessFile(_ string) error {
	if n.desiredError != nil {
		return *n.desiredError
	}
	return nil
}

func TestProcessChannel(t *testing.T) {
	worker := NewWorker(1, noOpCleaner{desiredError: nil})

	workerQueue := make(chan workerInput, 1)
	errorCh := make(chan error, 1)

	workerQueue <- "test.yaml"
	close(workerQueue)

	worker.ProcessQueue(workerQueue, errorCh)
	// TODO(thomas): this might actually be racy under some conditions, can this be done better?
	timer := time.NewTimer(time.Second)
	var err error
	select {
	case err = <-errorCh:
	case <-timer.C:
	}

	require.Nil(t, err)
}

func TestErrorPropagatesToChannel(t *testing.T) {
	desiredErr := errors.New("fail")
	worker := NewWorker(1, noOpCleaner{desiredError: &desiredErr})

	workerQueue := make(chan workerInput, 1)
	errorCh := make(chan error, 1)

	workerQueue <- "test.yaml"
	close(workerQueue)

	worker.ProcessQueue(workerQueue, errorCh)
	timer := time.NewTimer(time.Second)
	var err error
	select {
	case err = <-errorCh:
	case <-timer.C:
	}
	require.NotNil(t, err)
	require.Equal(t, &fileProcessingError{
		path:  "test.yaml",
		cause: desiredErr,
	}, err)

}
