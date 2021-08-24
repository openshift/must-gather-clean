package traversal

import (
	"path/filepath"
	"sort"
	"sync"

	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
)

type FileWalker struct {
	obfuscators []obfuscator.Obfuscator
	omitters    []omitter.Omitter
	reader      input.Inputter
	workerCount int
	workers     []*worker
	writer      output.Outputter
}

// NewFileWalker returns a FileWalker. It ensures that the reader directory exists and is readable.
func NewFileWalker(reader input.Inputter, writer output.Outputter, obfuscators []obfuscator.Obfuscator, omitters []omitter.Omitter, workerCount int) (*FileWalker, error) {
	return &FileWalker{
		obfuscators: obfuscators,
		omitters:    omitters,
		reader:      reader,
		workerCount: workerCount,
		writer:      writer,
	}, nil
}

// Traverse should be called to start processing the reader directory.
func (w *FileWalker) Traverse() {
	wg := sync.WaitGroup{}
	errorCh := make(chan error, w.workerCount)
	queue := make(chan workerFile, w.workerCount)
	w.workers = make([]*worker, w.workerCount)
	for i := 0; i < w.workerCount; i++ {
		wk := newWorker(i+1, w.obfuscators, w.omitters, queue, w.writer, errorCh)
		w.workers[i] = wk
		wg.Add(1)
		go func() {
			wk.run()
			wg.Done()
		}()
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

	w.processDir(w.reader.Root(), "", queue, errorCh)
	close(queue)
	wg.Wait()

	// once all the workers have exited close the error channel and wait for the exit goroutine to complete.
	close(errorCh)
	errorWg.Wait()
}

func (w *FileWalker) GenerateReport() *Report {
	report := &Report{}
	for _, o := range w.obfuscators {
		report.Replacements = append(report.Replacements, o.ReportingResult())
	}
	omittedFiles := make([]string, 0)
	for _, w := range w.workers {
		for of := range w.omittedFiles {
			omittedFiles = append(omittedFiles, of)
		}
	}
	// sorting helps humans review the file and ensuring test stability
	sort.Strings(omittedFiles)
	report.Omissions = omittedFiles
	return report
}

func (w *FileWalker) processDir(inputDir input.Directory, outputDirName string, queue chan<- workerFile, errorCh chan<- error) {
	// list all the entities in the directory
	entries, err := inputDir.Entries()
	if err != nil {
		errorCh <- &fileProcessingError{
			path:  inputDir.Path(),
			cause: err,
		}
		return
	}
	for _, entry := range entries {
		switch e := entry.(type) {
		case input.Directory:
			childDirOutput := filepath.Join(outputDirName, e.Name())
			w.processDir(e, childDirOutput, queue, errorCh)
		case input.File:
			queue <- workerFile{
				f:         e,
				outputDir: outputDirName,
			}
		}
	}
}
