package traversal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
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

type WorkerInput struct {
	// path here is relative to the must-gather root folder
	path string
}

type QueueProcessor interface {
	ProcessQueue(queue chan WorkerInput, errorCh chan error)
}

type Worker struct {
	id           int
	inputFolder  string
	outputFolder string
	obfuscators  []obfuscator.Obfuscator
	fileOmitters []omitter.FileOmitter
	k8sOmitters  []omitter.KubernetesResourceOmitter
	reporter     Reporter
}

func (w *Worker) ProcessQueue(queue chan WorkerInput, errorCh chan error) {
	for wf := range queue {
		klog.V(3).Infof("[Worker %02d] Processing %s\n", w.id, wf.path)

		// check if the file should be omitted
		omit, err := w.shouldOmitFile(wf.path)
		if err != nil {
			errorCh <- &fileProcessingError{
				path:  wf.path,
				cause: err,
			}
			continue
		}

		// If the file should be omitted then stop processing.
		if omit {
			w.reporter.ReportOmission(wf.path)
			klog.V(2).Infof("[Worker %02d] Omitting file %s", w.id, wf.path)
			continue
		}

		isKubernetesResource := true
		kubeResource, err := kube.ReadKubernetesResourceFromPath(filepath.Join(w.inputFolder, wf.path))
		if err != nil {
			if err == kube.NoKubernetesResourceError {
				isKubernetesResource = false
			} else {
				errorCh <- &fileProcessingError{
					path:  wf.path,
					cause: err,
				}
				continue
			}
		}

		if isKubernetesResource {
			omit, err := w.shouldOmitK8sResource(kubeResource)
			if err != nil {
				errorCh <- &fileProcessingError{
					path:  wf.path,
					cause: err,
				}
				continue
			}

			if omit {
				w.reporter.ReportOmission(wf.path)
				klog.V(2).Infof("[Worker %02d] Omitting k8s resource '%s'", w.id, wf.path)
				continue
			}
		}

		originalPath := wf.path
		newPath := originalPath
		for _, o := range w.obfuscators {
			newPath = o.Path(newPath)
		}

		if originalPath != newPath {
			klog.V(2).Infof("[Worker %02d] Obfuscating file '%s' as '%s'", w.id, originalPath, newPath)
		}

		err = w.obfuscateFileContent(wf.path, newPath)
		if err != nil {
			errorCh <- &fileProcessingError{
				path:  wf.path,
				cause: err,
			}
			continue
		}
		klog.V(3).Infof("[Worker %02d] Finished processing %s\n", w.id, wf.path)
	}
}

func (w *Worker) shouldOmitFile(path string) (bool, error) {
	for _, o := range w.fileOmitters {
		omit, err := o.Omit(path)
		if err != nil {
			return false, err
		}
		if omit {
			return true, nil
		}
	}
	return false, nil
}

func (w *Worker) shouldOmitK8sResource(resource *kube.ResourceList) (bool, error) {
	for _, o := range w.k8sOmitters {
		omit, err := o.Omit(resource)
		if err != nil {
			return false, err
		}
		if omit {
			return true, nil
		}
	}
	return false, nil
}

func (w *Worker) obfuscateFileContent(inputFile string, outputFile string) error {
	readPath := filepath.Join(w.inputFolder, inputFile)
	writePath := filepath.Join(w.outputFolder, outputFile)
	writePathParentDir := filepath.Dir(writePath)

	inputOsFile, err := os.Open(readPath)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(inputOsFile)

	err = os.MkdirAll(writePathParentDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", writePathParentDir, err)
	}

	outputOsFile, err := os.OpenFile(writePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(outputOsFile)
	for scanner.Scan() {
		contents := scanner.Text()
		for _, o := range w.obfuscators {
			contents = o.Contents(contents)
		}

		_, err = fmt.Fprintln(writer, contents)
		if err != nil {
			return err
		}
	}

	// we deliberately do not defer the closes, as this will be processed asynchronous and
	// any error will make the CLI exit immediately anyway

	err = inputOsFile.Close()
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	err = outputOsFile.Close()
	if err != nil {
		return err
	}

	return nil
}

func NewWorker(
	id int,
	inputFolder string,
	outputFolder string,
	obfuscators []obfuscator.Obfuscator,
	fileOmitters []omitter.FileOmitter,
	k8sOmitters []omitter.KubernetesResourceOmitter,
	reporter Reporter) QueueProcessor {

	return &Worker{
		id:           id,
		inputFolder:  inputFolder,
		outputFolder: outputFolder,
		obfuscators:  obfuscators,
		fileOmitters: fileOmitters,
		k8sOmitters:  k8sOmitters,
		reporter:     reporter,
	}
}
