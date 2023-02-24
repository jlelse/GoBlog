package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/highlighting"
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
			err = a.compileAsset(strings.TrimPrefix(path, assetsFolder+"/"), file)
			_ = file.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// Add syntax highlighting CSS
	return a.initChromaCSS()
}

func (a *goBlog) compileAsset(name string, read io.Reader) error {
	ext := path.Ext(name)
	switch ext {
	case ".js":
		read = a.min.Get().Reader(contenttype.JS, read)
	case ".css":
		read = a.min.Get().Reader(contenttype.CSS, read)
	case ".xml", ".xsl":
		read = a.min.Get().Reader(contenttype.XML, read)
	}
	// Read file
	hash := sha256.New()
	body, err := io.ReadAll(io.TeeReader(read, hash))
	if err != nil {
		return err
	}
	// File name
	compiledFileName := fmt.Sprintf("%x%s", hash.Sum(nil), ext)
	// Save file
	a.assetFiles[compiledFileName] = &assetFile{
		contentType: mime.TypeByExtension(ext),
		body:        body,
	}
	// Save mapping of original file name to compiled file name
	a.assetFileNames[name] = compiledFileName
	return err
}

// Function for templates
func (a *goBlog) assetFileName(fileName string) string {
	return "/" + a.assetFileNames[fileName]
}

func (a *goBlog) allAssetPaths() []string {
	paths := make([]string, 0)
	for _, name := range a.assetFileNames {
		paths = append(paths, "/"+name)
	}
	return paths
}

func (a *goBlog) checkTemplateAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		af, ok := a.assetFiles[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		a.serveAssetFile(w, af)
	})
}

// Gets only called by registered paths
func (a *goBlog) serveAsset(w http.ResponseWriter, r *http.Request) {
	af, ok := a.assetFiles[strings.TrimPrefix(r.URL.Path, "/")]
	if !ok {
		a.serve404(w, r)
		return
	}
	a.serveAssetFile(w, af)
}

func (*goBlog) serveAssetFile(w http.ResponseWriter, af *assetFile) {
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
	chromaStyle, err := highlighting.Style.Builder().Build()
	if err != nil {
		return err
	}
	// Generate and minify CSS
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(chromahtml.New(chromahtml.ClassPrefix("c-")).WriteCSS(pw, chromaStyle))
	}()
	err = a.compileAsset(chromaPath, pr)
	_ = pr.CloseWithError(err)
	return err
}
