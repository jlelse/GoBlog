package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_export(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.initMarkdown()

	err := app.db.savePost(&post{
		Path:    "/test/abc",
		Content: "ABC",
		Blog:    "en",
		Section: "test",
		Status:  statusDraft,
		Parameters: map[string][]string{
			"title": {"Title"},
		},
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	exportPath := filepath.Join(t.TempDir(), "export")
	err = app.exportMarkdownFiles(exportPath)
	require.NoError(t, err)

	exportFilePath := filepath.Join(exportPath, "/test/abc.md")
	require.FileExists(t, exportFilePath)

	fileContentBytes, err := os.ReadFile(exportFilePath)
	require.NoError(t, err)

	fileContent := string(fileContentBytes)
	assert.Contains(t, fileContent, `path: /test/abc`)
	assert.Contains(t, fileContent, `title: Title`)
	assert.Contains(t, fileContent, `updated: ""`)
	assert.Contains(t, fileContent, `published: ""`)
	assert.Contains(t, fileContent, `ABC`)

}
