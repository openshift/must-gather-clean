package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strconv"
)

func IsSymbolicLink(fileInfo fs.FileInfo) bool {
	return (fileInfo.Mode() & fs.ModeSymlink) == fs.ModeSymlink
}

// ResolveSymlink resolves every symlink component in path. Unlike
// filepath.EvalSymlinks, components after an existing prefix may be missing, so
// callers can create a new target reached through a dangling final symlink.
func ResolveSymlink(path string) (string, error) {
	const maxSymlinkDepth = 255
	originalPath := path
	volLen := len(filepath.VolumeName(path))
	if volLen < len(path) && os.IsPathSeparator(path[volLen]) {
		volLen++
	}
	volume := path[:volLen]
	destination := volume
	linksWalked := 0
	missingComponent := false

	var end int
	for start := volLen; start < len(path); start = end {
		for start < len(path) && os.IsPathSeparator(path[start]) {
			start++
		}
		end = start
		for end < len(path) && !os.IsPathSeparator(path[end]) {
			end++
		}
		if start == end {
			if missingComponent {
				return "", fmt.Errorf("cannot resolve directory path %s through a missing component", originalPath)
			}
			break
		}

		component := path[start:end]
		if component == "." {
			if missingComponent && end == len(path) {
				return "", fmt.Errorf("cannot resolve directory path %s through a missing component", originalPath)
			}
			continue
		}
		if component == ".." {
			if missingComponent {
				// Removing the missing component lexically would change the path
				// the kernel requires callers to create.
				return "", fmt.Errorf("cannot safely resolve %s after a missing component", originalPath)
			}
			var separator int
			for separator = len(destination) - 1; separator >= volLen; separator-- {
				if os.IsPathSeparator(destination[separator]) {
					break
				}
			}
			if separator < volLen || destination[separator+1:] == ".." {
				if len(destination) > volLen {
					destination += string(os.PathSeparator)
				}
				destination += ".."
			} else {
				destination = destination[:separator]
			}
			continue
		}

		if len(destination) > len(filepath.VolumeName(destination)) && !os.IsPathSeparator(destination[len(destination)-1]) {
			destination += string(os.PathSeparator)
		}
		destination += component

		info, err := os.Lstat(destination)
		if errors.Is(err, os.ErrNotExist) {
			// No later component can be a symlink until a parent is created.
			missingComponent = true
			continue
		}
		if err != nil {
			return "", err
		}
		if !IsSymbolicLink(info) {
			if !info.IsDir() && end < len(path) {
				return "", fmt.Errorf("%s is not a directory", destination)
			}
			continue
		}

		linksWalked++
		if linksWalked > maxSymlinkDepth {
			return "", fmt.Errorf("too many levels of symbolic links resolving %s", originalPath)
		}
		link, err := os.Readlink(destination)
		if err != nil {
			return "", err
		}
		path = link + path[end:]

		linkVolumeLength := len(filepath.VolumeName(link))
		if linkVolumeLength > 0 {
			if linkVolumeLength < len(link) && os.IsPathSeparator(link[linkVolumeLength]) {
				linkVolumeLength++
			}
			volume = link[:linkVolumeLength]
			destination = volume
			end = len(volume)
		} else if len(link) > 0 && os.IsPathSeparator(link[0]) {
			destination = link[:1]
			end = 1
			volume = link[:1]
			volLen = 1
		} else {
			var separator int
			for separator = len(destination) - 1; separator >= volLen; separator-- {
				if os.IsPathSeparator(destination[separator]) {
					break
				}
			}
			if separator < volLen {
				destination = volume
			} else {
				destination = destination[:separator]
			}
			end = 0
		}
	}

	// The lexical resolution above intentionally supports missing final targets.
	// Ask the kernel to validate the original path as well: trailing separators,
	// directory-shaped final components, host symlink limits, and permission errors
	// must not be reported as a successful resolution. A missing target remains
	// valid because callers may create it through a dangling final symlink.
	if _, err := os.Stat(originalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cannot resolve report path %s: %w", originalPath, err)
	}
	return filepath.Clean(destination), nil
}

func Relink(readPath string, writePath string, readPathStat os.FileInfo) error {
	src, err := os.Readlink(readPath)
	if err != nil {
		return fmt.Errorf("failed to read link in %s: %w", readPath, err)
	}

	err = os.Symlink(src, writePath)
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to link from %s to %s: %w", readPath, writePath, os.ErrExist)
	}

	klog.V(1).Infof("could not link '%s' to '%s', writing the link target instead. Error was: %v", src, writePath, err)

	outputOsFile, err := os.OpenFile(writePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, readPathStat.Mode())
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to link from %s to %s: %w", readPath, writePath, os.ErrExist)
		}
		return fmt.Errorf("failed to create and open '%s': %w", writePath, err)
	}

	_, err = outputOsFile.WriteString(src)
	if err != nil {
		_ = outputOsFile.Close()
		return fmt.Errorf("failed to write link target to '%s': %w", writePath, err)
	}

	err = outputOsFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close '%s': %w", writePath, err)
	}

	err = chown(writePath, readPathStat)
	if err != nil {
		return err
	}

	return nil
}

func EnsureInputOutputPath(inputPath string, outputPath string, deleteOutputFolder bool) error {
	_, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input folder does not exist: %w", err)
		}
		return fmt.Errorf("failed to stat input folder: %w", err)
	}

	err = ensureOutputPath(outputPath, deleteOutputFolder, inputPath)
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

// MkdirAllWithChown is a modified os.MkdirAll that creates perms according to the input folder hierarchy, from bottom to top
func MkdirAllWithChown(pathToCreate string, existingInputPath string) error {
	// short-cut in case the pathToCreate already exists
	_, err := os.Lstat(pathToCreate)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	// ensure the parent exists recursively
	parentPath := filepath.Dir(pathToCreate)
	_, err = os.Lstat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			inputParentPath := filepath.Dir(existingInputPath)
			err = MkdirAllWithChown(parentPath, inputParentPath)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	perm, err := os.Lstat(existingInputPath)
	if err != nil {
		return err
	}
	err = os.Mkdir(pathToCreate, perm.Mode())
	if err != nil {
		// might've been created by another goroutine in the meantime -- okay to proceed
		if !os.IsExist(err) {
			return err
		}
	}

	err = chown(pathToCreate, perm)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", pathToCreate, err)
	}
	return nil
}

func ensureOutputPath(path string, deleteIfExists bool, inputFolderPath string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return MkdirAllWithChown(path, inputFolderPath)
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

	return MkdirAllWithChown(path, inputFolderPath)
}
