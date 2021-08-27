package output

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Closer is used to close a file when done
type Closer func() error

// Outputter is an interface for any object which can write the processed output.
type Outputter interface {
	// Writer returns an io.StringWriter which can used to write to the file that will be created under as relativePath (yes, this is a file).
	// All paths up to the file are created automatically. Errors are returned when the folder can't be created or the file already exists.
	Writer(relativePath string, permissions os.FileMode) (Closer, io.StringWriter, error)
}

type fsWriter struct {
	outputDir string
}

func (f *fsWriter) Writer(relativePathToFile string, permissions os.FileMode) (Closer, io.StringWriter, error) {
	filePath := filepath.Join(f.outputDir, relativePathToFile)
	parentDir := filepath.Dir(filePath)
	err := os.MkdirAll(parentDir, 0755)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create directory %s: %w", parentDir, err)
	}
	_, err = os.Stat(filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to determine if %s already exists: %w", filePath, err)
	}
	if err == nil {
		return nil, nil, fmt.Errorf("file %s already exists", filePath)
	}
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, permissions)
	if err != nil {
		return nil, nil, err
	}
	writer := bufio.NewWriter(file)
	return func() error {
		if err := writer.Flush(); err != nil {
			return err
		}
		return file.Close()
	}, writer, nil
}

func EnsureOutputPath(path string, deleteIfExists bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(path, 0700)
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("output destination must be a directory: '%s'", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to get contents of output directory '%s': %w", path, err)
	}

	if len(entries) != 0 {
		if deleteIfExists {
			err = os.RemoveAll(path)
			if err != nil {
				return fmt.Errorf("error while deleting the output path '%s': %w", path, err)
			}
		} else {
			return fmt.Errorf("output directory %s is not empty", path)
		}
	}

	err = os.MkdirAll(path, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", path, err)
	}

	return nil
}

// NewFSWriter returns an Outputter which writes the files and directories to a specified location.
func NewFSWriter(path string) (Outputter, error) {
	return &fsWriter{outputDir: path}, nil
}
