package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

type fsFile struct {
	os.FileInfo
	path string
	root *fileSystemInput
}

func (f *fsFile) Path() string {
	return f.path
}

func (f *fsFile) Permissions() os.FileMode {
	return f.Mode().Perm()
}

func (f *fsFile) Scanner() (*bufio.Scanner, func() error, error) {
	file, err := os.Open(filepath.Join(f.root.Path(), f.path))
	if err != nil {
		return nil, nil, err
	}
	scanner := bufio.NewScanner(file)
	return scanner, file.Close, nil
}

func newFsFile(root *fileSystemInput, path string, info os.FileInfo) *fsFile {
	return &fsFile{root: root, path: path, FileInfo: info}
}

type fsDir struct {
	os.FileInfo
	path string
	root *fileSystemInput
}

func newFsDir(root *fileSystemInput, path string, info os.FileInfo) *fsDir {
	return &fsDir{root: root, path: path, FileInfo: info}
}

func (f *fsDir) Entries() ([]DirEntry, error) {
	entries, err := os.ReadDir(filepath.Join(f.root.Path(), f.path))
	if err != nil {
		return nil, err
	}
	var dirEntries []DirEntry
	for _, e := range entries {
		fileInfo, err := e.Info()
		if err != nil {
			return nil, err
		}
		// If file is a symlink then ignore it
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if fileInfo.IsDir() {
			dirEntries = append(dirEntries, newFsDir(f.root, filepath.Join(f.path, e.Name()), fileInfo))
		} else {
			dirEntries = append(dirEntries, newFsFile(f.root, filepath.Join(f.path, e.Name()), fileInfo))
		}
	}
	return dirEntries, nil
}

func (f *fsDir) Path() string {
	return f.path
}

type fileSystemInput struct {
	rootDir string
	info    os.FileInfo
}

func (f *fileSystemInput) Root() Directory {
	return newFsDir(f, "", f.info)
}

func (f *fileSystemInput) Path() string {
	return f.rootDir
}

func NewFSInput(inputDir string) (Inputter, error) {
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
	return &fileSystemInput{
		rootDir: inputPath,
		info:    fileInfo,
	}, nil
}
