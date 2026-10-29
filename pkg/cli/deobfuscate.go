package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/deobfuscator"
	"github.com/openshift/must-gather-clean/pkg/fsutil"
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

	mapping, err := deobfuscator.LoadReport(reportPath)
	if err != nil {
		return err
	}
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open support response %s: %w", inputPath, err)
	}
	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close support response %s: %w", inputPath, closeErr))
		}
	}()
	if err := rejectAliasedFiles(inputPath, reportPath, outputPath); err != nil {
		return err
	}

	return writeDeobfuscatedFile(mapping, inputFile, outputPath)
}

func rejectAliasedFiles(inputPath, reportPath, outputPath string) error {
	sameFile, err := pathsReferToSameFile(inputPath, reportPath)
	if err != nil {
		return fmt.Errorf("failed to compare support response and report files: %w", err)
	}
	if sameFile {
		return errors.New("input and report paths must differ")
	}

	sameFile, err = pathsReferToSameFile(inputPath, outputPath)
	if err != nil {
		return fmt.Errorf("failed to compare support response and output files: %w", err)
	}
	if sameFile {
		return errors.New("input and output paths must differ")
	}

	sameFile, err = pathsReferToSameFile(reportPath, outputPath)
	if err != nil {
		return fmt.Errorf("failed to compare report and output files: %w", err)
	}
	if sameFile {
		return errors.New("report and output paths must differ")
	}
	return nil
}

// writeDeobfuscatedFile writes to a temporary file beside the destination and
// publishes it only after reading and writing succeed.
func writeDeobfuscatedFile(mapping deobfuscator.Mapping, input io.Reader, outputPath string) (err error) {
	targetPath, err := fsutil.ResolveSymlink(outputPath)
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
			err = errors.Join(err, fmt.Errorf("failed to clean temporary output for %s: %w", outputPath, cleanupErr))
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
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("failed to publish deobfuscated support response %s: %w", outputPath, err)
	}
	return nil
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
