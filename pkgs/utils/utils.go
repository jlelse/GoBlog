package utils

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func SaveToFile(reader io.Reader, fileName string) error {
	return SaveToFileWithMode(reader, fileName, os.ModePerm, 0666)
}

func SaveToFileWithMode(reader io.Reader, fileName string, dirMode, fileMode fs.FileMode) (err error) {
	// Create folder path if not exists
	if err := os.MkdirAll(filepath.Dir(fileName), dirMode); err != nil {
		return err
	}
	// Create file
	out, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()
	// Copy response to file
	if _, err = io.Copy(out, reader); err != nil {
		return err
	}
	// Sync file to ensure all data is written to disk
	if err = out.Sync(); err != nil {
		return err
	}
	return nil
}
