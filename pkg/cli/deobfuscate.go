package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/deobfuscator"
)

// RunDeobfuscate restores values in input using the report at reportPath.
func RunDeobfuscate(reportPath string, input io.Reader, output io.Writer) error {
	mapping, err := deobfuscator.LoadReport(reportPath)
	if err != nil {
		return err
	}
	return runDeobfuscate(mapping, input, output)
}

func runDeobfuscate(mapping deobfuscator.Mapping, input io.Reader, output io.Writer) error {
	if err := mapping.ReplaceReader(input, output); err != nil {
		return fmt.Errorf("failed to deobfuscate support response: %w", err)
	}
	return nil
}

// RunDeobfuscateFile restores values from inputPath to outputPath.
func RunDeobfuscateFile(reportPath, inputPath, outputPath string) (err error) {
	if inputPath == "" || outputPath == "" {
		return fmt.Errorf("input and output file paths are required")
	}

	mapping, loadErr := deobfuscator.LoadReport(reportPath)
	if loadErr != nil {
		return loadErr
	}
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open support response %s: %w", inputPath, err)
	}
	defer func() {
		if inputFile != nil {
			if closeErr := inputFile.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("failed to close support response %s: %w", inputPath, closeErr)
			}
		}
	}()

	sameFile, statErr := pathsReferToSameFile(inputPath, reportPath)
	if statErr != nil {
		return fmt.Errorf("failed to compare support response and report files: %w", statErr)
	}
	if sameFile {
		return fmt.Errorf("input and report paths must differ")
	}

	sameFile, statErr = pathsReferToSameFile(inputPath, outputPath)
	if statErr != nil {
		return fmt.Errorf("failed to compare support response and output files: %w", statErr)
	}
	if sameFile {
		return fmt.Errorf("input and output paths must differ")
	}
	sameFile, statErr = pathsReferToSameFile(reportPath, outputPath)
	if statErr != nil {
		return fmt.Errorf("failed to compare report and output files: %w", statErr)
	}
	if sameFile {
		return fmt.Errorf("report and output paths must differ")
	}

	beforePublish := func() error {
		closeErr := inputFile.Close()
		inputFile = nil
		if closeErr != nil {
			return fmt.Errorf("failed to close support response %s: %w", inputPath, closeErr)
		}
		return nil
	}
	return writeDeobfuscatedFile(mapping, inputFile, outputPath, beforePublish)
}

// writeDeobfuscatedFile writes to a temporary file beside the destination and
// publishes it only after reading, writing, and closing the input all succeed.
func writeDeobfuscatedFile(mapping deobfuscator.Mapping, input io.Reader, outputPath string, beforePublish func() error) (err error) {
	targetPath, err := resolveOutputSymlink(outputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve output path %s: %w", outputPath, err)
	}

	outputDir := filepath.Dir(targetPath)
	temporaryDir, err := os.MkdirTemp(outputDir, "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary output for %s: %w", outputPath, err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(temporaryDir); cleanupErr != nil {
			cleanupErr = fmt.Errorf("failed to clean temporary output for %s: %w", outputPath, cleanupErr)
			err = errors.Join(err, cleanupErr)
		}
	}()
	temporaryPath := filepath.Join(temporaryDir, filepath.Base(targetPath))

	var existingMode os.FileMode
	existingOutput := false
	if info, statErr := os.Stat(targetPath); statErr == nil {
		existingOutput = true
		existingMode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect output path %s: %w", outputPath, statErr)
	}

	file, err := os.Create(temporaryPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary output for %s: %w", outputPath, err)
	}
	if existingOutput {
		if err := file.Chmod(existingMode); err != nil {
			_ = file.Close()
			return fmt.Errorf("failed to preserve output mode for %s: %w", outputPath, err)
		}
	}

	writeErr := runDeobfuscate(mapping, input, file)
	closeErr := file.Close()
	if writeErr != nil {
		if closeErr != nil {
			return fmt.Errorf("%w (also failed to close temporary output: %v)", writeErr, closeErr)
		}
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close temporary output for %s: %w", outputPath, closeErr)
	}
	if beforePublish != nil {
		if err := beforePublish(); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("failed to publish deobfuscated support response %s: %w", outputPath, err)
	}
	return nil
}

// resolveOutputSymlink keeps file-output behavior consistent with os.Create:
// write through a valid symlink instead of replacing the symlink itself.
func resolveOutputSymlink(outputPath string) (string, error) {
	info, err := os.Lstat(outputPath)
	if errors.Is(err, os.ErrNotExist) {
		return outputPath, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return outputPath, nil
	}
	return filepath.EvalSymlinks(outputPath)
}

func pathsReferToSameFile(firstPath, secondPath string) (bool, error) {
	firstInfo, err := os.Stat(firstPath)
	if err != nil {
		return false, err
	}
	secondInfo, err := os.Stat(secondPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(firstInfo, secondInfo), nil
}
