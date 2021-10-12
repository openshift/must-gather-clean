package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"k8s.io/klog/v2"
)

func IsSymbolicLink(fileInfo fs.FileInfo) bool {
	return (fileInfo.Mode() & fs.ModeSymlink) == fs.ModeSymlink
}

func Relink(readPath string, writePath string, readPathStat os.FileInfo) error {
	src, err := os.Readlink(readPath)
	if err != nil {
		return fmt.Errorf("failed to read link in %s: %w", readPath, err)
	}

	// we try to link once, if it fails on a link error we will try by shelling out, otherwise place a file with the original linkage
	err = os.Symlink(src, writePath)
	if err == nil {
		return nil
	} else {
		klog.V(1).Infof("could not link '%s' to '%s', trying shell instead. Error was: %w", src, writePath, err)
	}

	cmd := exec.Command("cp", "--preserve=links", "--no-dereference", src, writePath)
	err = cmd.Run()
	if err == nil {
		return nil
	} else if ee, ok := err.(*exec.ExitError); ok {
		klog.V(1).Infof("could not link '%s' to '%s' via shell, writing file instead. Error was: %w", src, writePath, ee)
	}

	err = os.WriteFile(writePath, []byte(src), readPathStat.Mode())
	if err != nil {
		return fmt.Errorf("failed to link from %s to %s: %w", readPath, writePath, err)
	}

	err = chown(writePath, readPathStat)
	if err != nil {
		return err
	}

	return nil
}

func CreateDirLikeInput(inputDir string, outputDir string) error {
	inputStat, err := os.Lstat(inputDir)
	if err != nil {
		return fmt.Errorf("failed to lstat input dir %s: %w", inputDir, err)
	}

	err = mkdirAllWithChown(outputDir, inputStat)
	if err != nil {
		return err
	}

	return nil
}

func EnsureInputOutputPath(inputPath string, outputPath string, deleteOutputFolder bool) error {
	inputStat, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input folder does not exist: %w", err)
		}
		return fmt.Errorf("failed to stat input folder: %w", err)
	}

	err = ensureOutputPath(outputPath, deleteOutputFolder, inputStat)
	if err != nil {
		return fmt.Errorf("failed to ensure output folder: %w", err)
	}

	return nil
}

func CreateNonConflictingFile(outputFilePath string, inputFileInfo os.FileInfo) (*os.File, error) {
	// we need to assess whether the file exists already to ensure we don't overwrite existing obfuscated data.
	// that can happen while obfuscating file names and their paths.
	// Additionally, the stat check is required because os.O_CREATE will implicitly os.O_TRUNC if a file already exist
	_, err := os.Lstat(outputFilePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to determine if %s already exists: %w", outputFilePath, err)
	}
	if err == nil {
		fileExt := 0
		for {
			fileExt++
			samplePath := outputFilePath + "." + strconv.Itoa(fileExt)
			_, err := os.Lstat(samplePath)
			if err != nil {
				if os.IsNotExist(err) {
					outputFilePath = samplePath
					break
				} else {
					return nil, fmt.Errorf("failed to determine if %s already exists: %w", samplePath, err)
				}
			}
		}
	}

	outputOsFile, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_WRONLY, inputFileInfo.Mode())
	if err != nil {
		return nil, fmt.Errorf("failed to create and open '%s': %w", outputFilePath, err)
	}

	err = chown(outputFilePath, inputFileInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to chown after opening '%s': %w", outputFilePath, err)
	}

	return outputOsFile, nil
}

func ensureOutputPath(path string, deleteIfExists bool, inputFolderStat os.FileInfo) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return mkdirAllWithChown(path, inputFolderStat)
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

	return mkdirAllWithChown(path, inputFolderStat)
}

// this is a modified os.MkdirAll that creates perms according to the input folder hierarchy, from bottom to top
func mkdirAllWithChown(path string, inputStat os.FileInfo) error {
	// short-cut in case the path already exists
	_, err := os.Lstat(path)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// ensure the parent exists recursively
	parentPath := filepath.Dir(path)
	_, err = os.Lstat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			inputParentPath := filepath.Dir(inputStat.Name())
			perm, err := os.Lstat(inputParentPath)
			if err != nil {
				return err
			}

			err = mkdirAllWithChown(parentPath, perm)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	err = os.Mkdir(path, inputStat.Mode())
	if err != nil {
		// might've been created by another goroutine in the meantime -- okay to proceed
		if !os.IsExist(err) {
			return err
		}
	}

	err = chown(path, inputStat)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

func chown(path string, stat fs.FileInfo) error {
	// there is no equivalent to chown in windows, thus we ignore it explicitly
	if runtime.GOOS != "windows" {
		uid := stat.Sys().(*syscall.Stat_t).Uid
		gid := stat.Sys().(*syscall.Stat_t).Gid
		err := os.Chown(path, int(uid), int(gid))
		if err != nil {
			return fmt.Errorf("failed to chown '%s' back to owner (%d, %d): %w", path, uid, gid, err)
		}
	}
	return nil
}
