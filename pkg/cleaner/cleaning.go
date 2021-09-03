package cleaner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/kube"
	"github.com/openshift/must-gather-clean/pkg/obfuscator"
	"github.com/openshift/must-gather-clean/pkg/omitter"
)

type Processor interface {
	// Process is the end2end method for cleaning (omit + obfuscate path and content of) a file on disk.
	// Will return nil if the file was processed without an error (e.g. through omission) or the error otherwise.
	Process(inputFile string) error
}

type ReadWriteObfuscator interface {
	// ObfuscateReader obfuscates on an agnostic line-based reader and writes to an agnostic writer facility
	ObfuscateReader(inputReader io.Reader, outputWriter io.Writer) error
}

type FileObfuscator interface {
	// ObfuscateFile obfuscates a text file and writes the result into the outputFile.
	ObfuscateFile(inputFile string, outputFile string) error
}

type ContentObfuscator struct {
	Obfuscator obfuscator.Obfuscator
}

type FileContentObfuscator struct {
	ContentObfuscator

	inputFolder  string
	outputFolder string
}

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

	// we need to assess whether the file exists already to ensure we don't overwrite existing obfuscated data.
	// that can happen while obfuscating file names and their paths.
	// Additionally, the stat check is required because os.O_CREATE will implicitly os.O_TRUNC if a file already exist
	_, err = os.Stat(writePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to determine if %s already exists: %w", writePath, err)
	}
	if err == nil {
		return fmt.Errorf("file %s already exists, check whether the obfuscators overwrite each other", writePath)
	}

	outputOsFile, err := os.OpenFile(writePath, os.O_CREATE|os.O_WRONLY, 0666)
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

func (c *ContentObfuscator) ObfuscateReader(inputReader io.Reader, outputWriter io.Writer) error {
	scanner := bufio.NewScanner(inputReader)
	writer := bufio.NewWriter(outputWriter)
	for scanner.Scan() {
		contents := c.Obfuscator.Contents(scanner.Text())

		_, err := fmt.Fprintln(writer, contents)
		if err != nil {
			return err
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
