package main

import (
	"crypto/sha1"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"git.jlel.se/jlelse/GoBlog/pkgs/contenttype"
)

const assetsFolder = "templates/assets"

type assetFile struct {
	contentType string
	body        []byte
}

func (a *goBlog) initTemplateAssets() (err error) {
	a.assetFileNames = map[string]string{}
	a.assetFiles = map[string]*assetFile{}
	err = filepath.Walk(assetsFolder, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			compiled, err := a.compileAsset(path)
			if err != nil {
				return err
			}
			if compiled != "" {
				a.assetFileNames[strings.TrimPrefix(path, assetsFolder+"/")] = compiled
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *goBlog) compileAsset(name string) (string, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	ext := path.Ext(name)
	compiledExt := ext
	m := a.min.Get()
	switch ext {
	case ".js":
		content, err = m.Bytes("application/javascript", content)
		if err != nil {
			return "", err
		}
	case ".css":
		content, err = m.Bytes("text/css", content)
		if err != nil {
			return "", err
		}
	default:
		// Do nothing
	}
	// Hashes
	sha1Hash := sha1.New()
	if _, err := sha1Hash.Write(content); err != nil {
		return "", err
	}
	// File name
	compiledFileName := fmt.Sprintf("%x", sha1Hash.Sum(nil)) + compiledExt
	// Create struct
	a.assetFiles[compiledFileName] = &assetFile{
		contentType: mime.TypeByExtension(compiledExt),
		body:        content,
	}
	return compiledFileName, err
}

// Function for templates
func (a *goBlog) assetFileName(fileName string) string {
	return "/" + a.assetFileNames[fileName]
}

func (a *goBlog) allAssetPaths() []string {
	var paths []string
	for _, name := range a.assetFileNames {
		paths = append(paths, "/"+name)
	}
	return paths
}

// Gets only called by registered paths
func (a *goBlog) serveAsset(w http.ResponseWriter, r *http.Request) {
	af, ok := a.assetFiles[strings.TrimPrefix(r.URL.Path, "/")]
	if !ok {
		a.serve404(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
	w.Header().Set(contentType, af.contentType+contenttype.CharsetUtf8Suffix)
	_, _ = w.Write(af.body)
}
