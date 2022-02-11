package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"go.goblog.app/app/pkgs/contenttype"
)

const assetsFolder = "templates/assets"

type assetFile struct {
	contentType string
	body        []byte
}

func (a *goBlog) initTemplateAssets() error {
	a.assetFileNames = map[string]string{}
	a.assetFiles = map[string]*assetFile{}
	if err := filepath.Walk(assetsFolder, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			// Open file
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			// Compile asset and close file
			compiled, err := a.compileAsset(path, file)
			_ = file.Close()
			if err != nil {
				return err
			}
			// Add to map
			if compiled != "" {
				a.assetFileNames[strings.TrimPrefix(path, assetsFolder+"/")] = compiled
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// Add syntax highlighting CSS
	if err := a.initChromaCSS(); err != nil {
		return err
	}
	return nil
}

func (a *goBlog) compileAsset(name string, read io.Reader) (string, error) {
	ext := path.Ext(name)
	switch ext {
	case ".js":
		read = a.min.Reader(contenttype.JS, read)
	case ".css":
		read = a.min.Reader(contenttype.CSS, read)
	case ".xml", ".xsl":
		read = a.min.Reader(contenttype.XML, read)
	}
	// Read file
	hash := sha256.New()
	body, err := io.ReadAll(io.TeeReader(read, hash))
	if err != nil {
		return "", err
	}
	// File name
	compiledFileName := fmt.Sprintf("%x%s", hash.Sum(nil), ext)
	// Create struct
	a.assetFiles[compiledFileName] = &assetFile{
		contentType: mime.TypeByExtension(ext),
		body:        body,
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
	w.Header().Set(cacheControl, "public,max-age=31536000,immutable")
	w.Header().Set(contentType, af.contentType+contenttype.CharsetUtf8Suffix)
	_, _ = w.Write(af.body)
}

func (a *goBlog) initChromaCSS() error {
	chromaPath := "css/chroma.css"
	// Check if file already exists
	if _, ok := a.assetFiles[chromaPath]; ok {
		return nil
	}
	// Initialize the style
	chromaStyleBuilder := chromaGoBlogStyle.Builder()
	chromaStyle, err := chromaStyleBuilder.Build()
	if err != nil {
		return err
	}
	// Write the CSS to a buffer
	var cssBuffer bytes.Buffer
	if err = chromahtml.New(chromahtml.ClassPrefix("c-")).WriteCSS(&cssBuffer, chromaStyle); err != nil {
		return err
	}
	// Compile asset
	compiled, err := a.compileAsset(chromaPath, &cssBuffer)
	if err != nil {
		return err
	}
	// Add to map
	a.assetFileNames[chromaPath] = compiled
	return nil
}
