package main

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const assetsFolder = "templates/assets"

var assetFileNames map[string]string = map[string]string{}
var assetFiles map[string]*assetFile = map[string]*assetFile{}

type assetFile struct {
	contentType string
	body        []byte
}

func initTemplateAssets() (err error) {
	err = filepath.Walk(assetsFolder, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			compiled, err := compileAsset(path)
			if err != nil {
				return err
			}
			if compiled != "" {
				assetFileNames[strings.TrimPrefix(path, assetsFolder+"/")] = compiled
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func compileAsset(name string) (string, error) {
	originalContent, err := ioutil.ReadFile(name)
	if err != nil {
		return "", err
	}
	ext := path.Ext(name)
	var compiledContent []byte
	compiledExt := ext
	switch ext {
	case ".js":
		compiledContent, err = minifier.Bytes("application/javascript", originalContent)
		if err != nil {
			return "", err
		}
	case ".css":
		compiledContent, err = minifier.Bytes("text/css", originalContent)
		if err != nil {
			return "", err
		}
	default:
		// Just copy the file
		compiledContent = originalContent
	}
	sha := sha1.New()
	if _, err := sha.Write(compiledContent); err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", sha.Sum(nil))
	compiledFileName := hash + compiledExt
	assetFiles[compiledFileName] = &assetFile{
		contentType: mime.TypeByExtension(compiledExt),
		body:        compiledContent,
	}
	return compiledFileName, err
}

// Function for templates
func assetFileName(fileName string) string {
	return "/" + assetFileNames[fileName]
}

func allAssetPaths() []string {
	var paths []string
	for _, name := range assetFileNames {
		paths = append(paths, "/"+name)
	}
	return paths
}

// Gets only called by registered paths
func serveAsset(w http.ResponseWriter, r *http.Request) {
	af, ok := assetFiles[strings.TrimPrefix(r.URL.Path, "/")]
	if !ok {
		serve404(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
	w.Header().Set(contentType, af.contentType+charsetUtf8Suffix)
	_, _ = w.Write(af.body)
}
