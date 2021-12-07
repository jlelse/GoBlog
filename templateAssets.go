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

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"go.goblog.app/app/pkgs/contenttype"
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
	// Add syntax highlighting CSS
	err = a.initChromaCSS()
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
		content, err = m.Bytes(contenttype.JS, content)
		if err != nil {
			return "", err
		}
	case ".css":
		content, err = m.Bytes(contenttype.CSS, content)
		if err != nil {
			return "", err
		}
	case ".xml", ".xsl":
		content, err = m.Bytes(contenttype.XML, content)
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

func (a *goBlog) initChromaCSS() error {
	// Check if file already exists
	if _, ok := a.assetFiles["css/chroma.css"]; ok {
		return nil
	}
	// Initialize the style
	chromaStyleBuilder := chromaGoBlogStyle.Builder()
	chromaStyle, err := chromaStyleBuilder.Build()
	if err != nil {
		return err
	}
	// Create a temporary file
	chromaTempFile, err := os.CreateTemp("", "chroma-*.css")
	if err != nil {
		return err
	}
	chromaTempFileName := chromaTempFile.Name()
	// Write the CSS to the file
	err = chromahtml.New(chromahtml.ClassPrefix("c-")).WriteCSS(chromaTempFile, chromaStyle)
	if err != nil {
		return err
	}
	// Close the file
	_ = chromaTempFile.Close()
	// Compile asset
	compiled, err := a.compileAsset(chromaTempFileName)
	_ = os.Remove(chromaTempFileName)
	if err != nil {
		return err
	}
	a.assetFileNames["css/chroma.css"] = compiled
	return nil
}
