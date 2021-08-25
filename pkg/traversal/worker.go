package traversal

import (
	"fmt"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
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

type workerFile struct {
	f         input.File
	outputDir string
}

type worker struct {
	id           int
	obfuscators  []obfuscator.Obfuscator
	fileOmitters []omitter.FileOmitter
	k8sOmitters  []omitter.KubernetesResourceOmitter
	queue        <-chan workerFile
	omittedFiles map[string]struct{}
	writer       output.Outputter
	errorCh      chan<- error
}

func newWorker(id int,
	obfuscators []obfuscator.Obfuscator,
	fileOmitters []omitter.FileOmitter,
	k8sOmitters []omitter.KubernetesResourceOmitter,
	queue <-chan workerFile,
	writer output.Outputter,
	errorCh chan<- error) *worker {
	return &worker{
		id:           id,
		obfuscators:  obfuscators,
		omittedFiles: map[string]struct{}{},
		fileOmitters: fileOmitters,
		k8sOmitters:  k8sOmitters,
		queue:        queue,
		writer:       writer,
		errorCh:      errorCh,
	}
}

func (w *worker) run() {
	for wf := range w.queue {
		klog.V(3).Infof("[worker %02d] Processing %s\n", w.id, wf.f.Path())

		// check if the file should be omitted
		omit, err := w.shouldOmitFile(wf.f)
		if err != nil {
			w.errorCh <- &fileProcessingError{
				path:  wf.f.Path(),
				cause: err,
			}
			continue
		}

		// If the file should be omitted then stop processing.
		if omit {
			w.omittedFiles[wf.f.Path()] = struct{}{}
			klog.V(2).Infof("[worker %02d] Omitting file %s", w.id, wf.f.Path())
			continue
		}

		isKubernetesResource := true
		kubeResource, err := kube.ReadKubernetesResourceFromPath(wf.f.AbsPath())
		if err != nil {
			if err == kube.NoKubernetesResourceError {
				isKubernetesResource = false
			} else {
				w.errorCh <- &fileProcessingError{
					path:  wf.f.Path(),
					cause: err,
				}
			}
		}

		if isKubernetesResource {
			omit, err := w.shouldOmitK8sResource(kubeResource)
			if err != nil {
				w.errorCh <- &fileProcessingError{
					path:  wf.f.Path(),
					cause: err,
				}
				continue
			}

			if omit {
				w.omittedFiles[wf.f.Path()] = struct{}{}
				klog.V(2).Infof("[worker %02d] Omitting k8s resource %s", w.id, wf.f.Path())
				continue
			}
		}

		// obfuscate the name if required
		newName := wf.f.Name()
		for _, o := range w.obfuscators {
			newName = o.FileName(newName)
		}

		if wf.f.Name() != newName {
			klog.V(2).Infof("[worker %02d] Obfuscating file %s as %s", w.id, wf.f.Name(), newName)
		}

		err = w.obfuscateFile(wf, newName)
		if err != nil {
			w.errorCh <- &fileProcessingError{
				path:  wf.f.Path(),
				cause: err,
			}
		}
		klog.V(3).Infof("[worker %02d] Finished processing %s\n", w.id, wf.f.Path())
	}
}

func (w *worker) shouldOmitFile(f input.File) (bool, error) {
	for _, o := range w.fileOmitters {
		omit, err := o.Omit(f.Name(), f.Path())
		if err != nil {
			return false, err
		}
		if omit {
			return true, nil
		}
	}
	return false, nil
}

func (w *worker) shouldOmitK8sResource(resource *kube.ResourceList) (bool, error) {
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

func (w *worker) obfuscateFile(wf workerFile, outputFileName string) error {
	closeWriter, writer, err := w.writer.Writer(wf.outputDir, outputFileName, wf.f.Permissions())
	if err != nil {
		return err
	}
	// close the output file when done
	defer func() {
		if err := closeWriter(); err != nil {
			w.errorCh <- &fileProcessingError{
				path:  wf.f.Path(),
				cause: err,
			}
		}
	}()

	scanner, closeReader, err := wf.f.Scanner()
	if err != nil {
		return err
	}
	defer func() {
		if err := closeReader(); err != nil {
			w.errorCh <- &fileProcessingError{
				path:  wf.f.Path(),
				cause: err,
			}
		}
	}()
	for scanner.Scan() {
		contents := scanner.Text()
		for _, o := range w.obfuscators {
			contents = o.Contents(contents)
		}
		_, err = writer.WriteString(fmt.Sprintf("%s\n", contents))
		if err != nil {
			return err
		}
	}
	return nil
}
