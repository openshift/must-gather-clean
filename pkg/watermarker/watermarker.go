package watermarking

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	version "github.com/openshift/must-gather-clean/pkg/version"
)

type WaterMarker interface {
	// WriteWaterMarkFile creates a watermark file in the specified path
	WriteWaterMarkFile(path string) error
}

type SimpleWaterMarker struct{}

func NewSimpleWaterMarker() *SimpleWaterMarker {
	return &SimpleWaterMarker{}
}

func (s *SimpleWaterMarker) WriteWaterMarkFile(path string) error {
	contents := time.Now().UTC().String() + " - " + version.GetVersion().Version
	err := os.WriteFile(filepath.Join(path, "watermark.txt"), []byte(contents), 0644)
	if err != nil {
		return fmt.Errorf("failed to create watermark file in output folder: %w", err)
	}
	return nil
}
