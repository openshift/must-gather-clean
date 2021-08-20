package traversal

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
	"k8s.io/klog/v2"
)

type FileWalker struct {
	reader       input.Inputter
	obfuscators  []obfuscator.Obfuscator
	omitters     []omitter.Omitter
	writer       output.Outputter
	omittedFiles map[string]struct{}
}

// NewFileWalker returns a FileWalker. It ensures that the reader directory exists and is readable.
func NewFileWalker(reader input.Inputter, writer output.Outputter, obfuscators []obfuscator.Obfuscator, omitters []omitter.Omitter) (*FileWalker, error) {
	return &FileWalker{reader: reader, obfuscators: obfuscators, writer: writer, omitters: omitters, omittedFiles: map[string]struct{}{}}, nil
}

// Traverse should be called to start processing the reader directory.
func (w *FileWalker) Traverse() error {
	return w.processDir(w.reader.Root(), "")
}

func (w *FileWalker) GenerateReport() *Report {
	report := &Report{}
	for _, o := range w.obfuscators {
		report.Replacements = append(report.Replacements, o.ReportingResult())
	}
	omittedFiles := make([]string, len(w.omittedFiles))
	var count int
	for of := range w.omittedFiles {
		omittedFiles[count] = of
		count++
	}
	// sorting helps humans review the file and ensuring test stability
	sort.Strings(omittedFiles)
	report.Omissions = omittedFiles
	return report
}

func (w *FileWalker) processDir(inputDir input.Directory, outputDirName string) error {
	// list all the entities in the directory
	entries, err := inputDir.Entries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch e := entry.(type) {
		case input.Directory:
			childDirOutput := filepath.Join(outputDirName, e.Name())
			err = w.processDir(e, childDirOutput)
			if err != nil {
				return err
			}
		case input.File:

			var omit bool
			// verify if the file should be omitted
			for _, o := range w.omitters {
				omit, err = o.File(e.Name(), e.Path())
				if err != nil {
					return fmt.Errorf("failed to determine if %s should be omitted based on path: %w", e.Path(), err)
				}
				if omit {
					w.omittedFiles[e.Path()] = struct{}{}
					break
				}
				omit, err = o.Contents(e.AbsPath())
				if err != nil {
					return fmt.Errorf("failed to determine if %s should be omitted based on contents: %w", e.Path(), err)
				}
				if omit {
					w.omittedFiles[e.Path()] = struct{}{}
					break
				}
			}
			// If the file should be omitted then stop processing.
			if omit {
				klog.V(2).Infof("omitting '%s'", e.Path())
				continue
			}

			// obfuscate the name if required
			newName := e.Name()
			for _, o := range w.obfuscators {
				newName = o.FileName(newName)
				klog.V(2).Infof("obfuscating filename '%s' to '%s'", e.Name(), newName)
			}

			err := func() error {
				writeCloser, writer, err := w.writer.Writer(outputDirName, newName, e.Permissions())
				if err != nil {
					return err
				}
				// close the output file when done
				defer func() {
					if err := writeCloser(); err != nil {
						klog.Exitf("failed to successfully write file %s: %v", filepath.Join(outputDirName, newName), err)
					}
				}()

				scanner, closeReader, err := e.Scanner()
				if err != nil {
					return err
				}
				defer func() {
					if err := closeReader(); err != nil {
						klog.Exitf("failed to close file %s after reading: %v", e.Path(), err)
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
			}()
			if err != nil {
				return err
			}

			klog.V(2).Infof("done processing '%s'", e.Path())
		}
	}
	return nil
}
