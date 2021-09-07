package pkg

import (
	"fmt"
	"runtime"
)

var (
	// versionFromGit is the constant representing the version of the must-gather-clean binary
	versionFromGit = "unknown"
	// commitFromGit is a constant representing the source version that
	// generated this build. It should be set during build via -ldflags.
	commitFromGit string
	// buildDate in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	buildDate string
)

type Version struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoOs      string `json:"goOs"`
	GoArch    string `json:"goArch"`
}

func GetVersion() Version {
	return Version{
		Version:   versionFromGit,
		GitCommit: commitFromGit,
		BuildDate: buildDate,
		GoOs:      runtime.GOOS,
		GoArch:    runtime.GOARCH,
	}
}

func (v Version) Print() {
	fmt.Printf("Version: %#v\n", v)
}
