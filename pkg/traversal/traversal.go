package traversal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
	"github.com/openshift/must-gather-clean/pkg/output"
)

type FileWalker struct {
	input       string
	obfuscators []obfuscator.Obfuscator
	writer      output.Outputter
	omitters    []omitter.Omitter
}

// NewFileWalker returns a FileWalker. It ensures that the input directory exists and is readable.
func NewFileWalker(inputDir string, writer output.Outputter, obfuscators []obfuscator.Obfuscator, omitters []omitter.Omitter) (*FileWalker, error) {
	fileInfo, err := os.Stat(inputDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access input directory %s: %w", inputDir, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("input path %s is not a directory", inputDir)
	}
	inputPath, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	return &FileWalker{input: inputPath, obfuscators: obfuscators, writer: writer, omitters: omitters}, nil
}

// Traverse should be called to start processing the input directory.
func (w *FileWalker) Traverse() error {
	return w.processDir(w.input, "")
}

func (w *FileWalker) processDir(inputDirName string, outputDirName string) error {
	// list all the entities in the directory
	entries, err := os.ReadDir(inputDirName)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fileInfo, err := e.Info()
		if err != nil {
			return err
		}

		// If a file is a symlink then ignore it
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if e.IsDir() {
			childDirOutput := filepath.Join(outputDirName, e.Name())
			childDirInput := filepath.Join(inputDirName, e.Name())
			err = w.processDir(childDirInput, childDirOutput)
			if err != nil {
				return err
			}
		} else {
			leafPath := filepath.Join(inputDirName, e.Name())

			var omit bool
			// verify if the file should be omitted
			for _, o := range w.omitters {
				omit, err = o.File(e.Name(), leafPath)
				if err != nil {
					return fmt.Errorf("failed to determine if %s should be omitted based on name: %w", leafPath, err)
				}
				if omit {
					break
				}
				omit, err = o.Contents(leafPath)
				if err != nil {
					return fmt.Errorf("failed to determine if %s should be omitted based on contents: %w", leafPath, err)
				}
				if omit {
					break
				}
			}
			// If the file should be omitted then stop processing.
			if omit {
				continue
			}
			return func() error {
				writeCloser, writer, err := w.writer.Writer(outputDirName, e.Name(), fileInfo.Mode())
				defer func() {
					if err := writeCloser(); err != nil {
						fmt.Printf("failed to successfully write file %s: %v", filepath.Join(outputDirName, e.Name()), err)
					}
				}()
				if err != nil {
					return err
				}
				file, err := os.Open(leafPath)
				if err != nil {
					return err
				}
				scanner := bufio.NewScanner(file)
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
		}
	}
	return nil
}
