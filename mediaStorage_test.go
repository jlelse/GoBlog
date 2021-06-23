package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_localMediaStorage(t *testing.T) {
	testDir := t.TempDir()

	l := &localMediaStorage{
		mediaURL: "https://example.com",
		path:     testDir,
	}

	testFileContent := "This is a test"

	loc, err := l.save("test.txt", strings.NewReader(testFileContent))
	require.Nil(t, err)
	assert.Equal(t, "https://example.com/test.txt", loc)

	file, err := os.ReadFile(filepath.Join(testDir, "test.txt"))
	require.Nil(t, err)
	assert.Equal(t, testFileContent, string(file))
}
