// Package cleaner contains all the business logic of cleaning (obfuscation + omission) files on different types of content (k8s resources, text files, paths).
package cleaner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
)

// Processor is responsible for processing a single input file.
type Processor interface {
	// Process is the end2end method for cleaning (omit + obfuscate path and content of) a file on disk.
	// Will return nil if the file was processed without an error (e.g. through omission) or the error otherwise.
	Process(inputFile string) error
}

// ReadWriteObfuscator is responsible for obfuscating input from an io.Reader and outputting to an io.Writer
type ReadWriteObfuscator interface {
	// ObfuscateReader obfuscates on an agnostic line-based reader and writes to an agnostic writer facility
	ObfuscateReader(inputReader io.Reader, outputWriter io.Writer) error
}

// FileObfuscator is responsible for obfuscating a file on the filesystem and output the result into another file.
type FileObfuscator interface {
	// ObfuscateFile obfuscates a text file and writes the result into the outputFile.
	ObfuscateFile(inputFile string, outputFile string) error
}

// ContentObfuscator wraps any obfuscator and implements ReadWriteObfuscator
type ContentObfuscator struct {
	Obfuscator obfuscator.Obfuscator
}

// FileContentObfuscator obfuscates a file by implementing FileObfuscator and ReadWriteObfuscator.
type FileContentObfuscator struct {
	ContentObfuscator

	inputFolder  string
	outputFolder string
	// defining a lock to avoid collisions while creating the files in a multi-threaded environment
	pathCollisionMutex sync.Mutex
}

// FileProcessor cleans (either omit or obfuscates) a path by implementing Processor.
type FileProcessor struct {
	FileContentObfuscator

	omitter omitter.Omitter
}

func (c *FileProcessor) Process(path string) error {
	omit, err := c.omitter.OmitPath(path)
	if err != nil {
		return err
	}

	if omit {
		return nil
	}

	isKubernetesResource := true
	kubeResource, err := kube.ReadKubernetesResourceFromPath(filepath.Join(c.inputFolder, path))
	if err != nil {
		if err == kube.NoKubernetesResourceError {
			isKubernetesResource = false
		} else {
			return err
		}
	}

	if isKubernetesResource {
		omit, err := c.omitter.OmitKubeResource(kubeResource)
		if err != nil {
			return err
		}

		if omit {
			return nil
		}
	}

	// obfuscate the text file with updated path name, which can also contain confidential information
	return c.ObfuscateFile(path, c.FileContentObfuscator.Obfuscator.Path(path))
}

func (c *FileContentObfuscator) ObfuscateFile(inputFile string, outputFile string) error {
	readPath := filepath.Join(c.inputFolder, inputFile)
	writePath := filepath.Join(c.outputFolder, outputFile)
	writePathParentDir := filepath.Dir(writePath)

	inputOsFile, err := os.Open(readPath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(writePathParentDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %c", writePathParentDir, err)
	}

	outputOsFile, err := c.generateNewFile(writePath)
	if err != nil {
		return fmt.Errorf("failed to create and open '%s': %w", writePath, err)
	}

	err = c.ObfuscateReader(inputOsFile, outputOsFile)
	if err != nil {
		return fmt.Errorf("failed to obfuscate input file '%s': %w", readPath, err)
	}

	err = inputOsFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close input file '%s': %w", readPath, err)
	}

	err = outputOsFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close output file '%s': %w", writePath, err)
	}

	return nil
}

// generateNewFile takes the inputFilePath as an argument, validates the existence of the inputFilePath
// If the file exists, this method creates a new file path with the ascending number pattern getting appended to the inputFilePath
// This method returns the newly created file
// Ex: If the inputFilePath is "/tmp/aml" and if the file exists, this method generates "/tmp/xyz.yaml.1".
func (c *FileContentObfuscator) generateNewFile(inputFilePath string) (*os.File, error) {
	c.pathCollisionMutex.Lock()
	defer c.pathCollisionMutex.Unlock()

	// we need to assess whether the file exists already to ensure we don't overwrite existing obfuscated data.
	// that can happen while obfuscating file names and their paths.
	// Additionally, the stat check is required because os.O_CREATE will implicitly os.O_TRUNC if a file already exist

	_, err := os.Stat(inputFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to determine if %s already exists: %w", inputFilePath, err)
	}
	if err == nil {
		fileExt := 0
		for {
			fileExt++
			samplePath := inputFilePath + "." + strconv.Itoa(fileExt)
			_, err := os.Stat(samplePath)
			if err != nil {
				if os.IsNotExist(err) {
					inputFilePath = samplePath
					break
				} else {
					return nil, fmt.Errorf("failed to determine if %s already exists: %w", samplePath, err)
				}
			}
		}
	}

	outputOsFile, err := os.OpenFile(inputFilePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to create and open '%s': %w", inputFilePath, err)
	}
	return outputOsFile, nil
}

func (c *ContentObfuscator) ObfuscateReader(inputReader io.Reader, outputWriter io.Writer) error {
	// we don't use bufio.Scanner anymore, since that can not read larger than 4096 byte lines (found in prometheus rules.json)
	reader := bufio.NewReader(inputReader)
	writer := bufio.NewWriter(outputWriter)

	for {
		isEOF := false
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if line == "" {
					break
				} else {
					isEOF = true
				}
			} else {
				return err
			}
		}

		contents := c.Obfuscator.Contents(line)

		_, err = fmt.Fprint(writer, contents)
		if err != nil {
			return err
		}

		if isEOF {
			break
		}
	}

	return writer.Flush()
}

func NewFileCleaner(inputPath string, outputPath string, obfuscator obfuscator.Obfuscator, omitter omitter.Omitter) Processor {
	return &FileProcessor{
		FileContentObfuscator: FileContentObfuscator{
			ContentObfuscator: ContentObfuscator{Obfuscator: obfuscator},
			inputFolder:       inputPath,
			outputFolder:      outputPath,
		},
		omitter: omitter,
	}
}
