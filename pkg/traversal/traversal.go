package traversal

import (
	"fmt"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/input"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
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
				omit, err = o.Contents(e.Path())
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
				continue
			}

			// obfuscate the name if required
			newName := e.Name()
			for _, o := range w.obfuscators {
				newName = o.FileName(newName)
			}

			err := func() error {
				writeCloser, writer, err := w.writer.Writer(outputDirName, newName, e.Permissions())
				if err != nil {
					return err
				}
				// close the output file when done
				defer func() {
					if err := writeCloser(); err != nil {
						fmt.Printf("failed to successfully write file %s: %v", filepath.Join(outputDirName, newName), err)
					}
				}()

				scanner, closeReader, err := e.Scanner()
				if err != nil {
					return err
				}
				defer func() {
					if err := closeReader(); err != nil {
						fmt.Printf("failed to close file %s after reading: %v", e.Path(), err)
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
		}
	}
	return nil
}
