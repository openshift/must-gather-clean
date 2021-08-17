package input

import (
	"bufio"
	"os"
)

type Directory interface {
	Entries() ([]DirEntry, error)
	Name() string
	Path() string
}

type File interface {
	Path() string
	Name() string
	Permissions() os.FileMode
	Scanner() (*bufio.Scanner, func() error, error)
	// AbsPath returns the absolute path to the file
	AbsPath() string
}

type DirEntry interface {
	IsDir() bool
}

type Inputter interface {
	Root() Directory
}