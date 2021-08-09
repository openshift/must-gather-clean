package schema

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidFiles(t *testing.T) {
	glob, err := filepath.Glob("testfiles/valid/*")
	assert.Nil(t, err)

	for _, validFile := range glob {
		_, err := ReadConfigFromPath(validFile)
		assert.Nilf(t, err, "unexpected error while reading valid config in %s", validFile)
	}
}

func TestInvalidFiles(t *testing.T) {
	glob, err := filepath.Glob("testfiles/malformed/*")
	assert.Nil(t, err)

	for _, invalidFile := range glob {
		_, err := ReadConfigFromPath(invalidFile)
		assert.NotNilf(t, err, "expected error while reading malformed config in %s", invalidFile)
	}
}

func TestFailsOnUnsupportedExtension(t *testing.T) {
	_, err := ReadConfigFromPath("schema_test.go")
	assert.Equal(t, wrapError(UnsupportedFileTypeError{UsedExtension: ".go", SupportedExtensions: supportedExtensions}), err)
}
