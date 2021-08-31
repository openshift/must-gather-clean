package traversal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type collectingQueueProcessor struct {
	paths []string
}

func (c *collectingQueueProcessor) ProcessQueue(queue chan WorkerInput, _ chan error) {
	for wf := range queue {
		c.paths = append(c.paths, wf.path)
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

			walker.Traverse()

			assert.Equal(t, tc.expectedResult, queueProc.paths)
		})
	}
}
