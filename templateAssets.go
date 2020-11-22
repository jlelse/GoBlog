package main

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const assetsFolder = "templates/assets"

var compiledAssetsFolder string
var assetFiles map[string]string

func initTemplateAssets() (err error) {
	compiledAssetsFolder, err = ioutil.TempDir("", "goblog-assets-*")
	if err != nil {
		return
	}
	assetFiles = map[string]string{}
	err = filepath.Walk(assetsFolder, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			compiled, err := compileAssets(path)
			if err != nil {
				return err
			}
			if compiled != "" {
				assetFiles[strings.TrimPrefix(path, assetsFolder+"/")] = compiled
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func compileAssets(name string) (compiledFileName string, err error) {
	originalContent, err := ioutil.ReadFile(name)
	if err != nil {
		return
	}
	ext := path.Ext(name)
	var compiledContent []byte
	compiledExt := ext
	switch ext {
	case ".js":
		compiledContent, err = minifier.Bytes("application/javascript", originalContent)
		if err != nil {
			return
		}
	case ".css":
		compiledContent, err = minifier.Bytes("text/css", originalContent)
		if err != nil {
			return
		}
	default:
		// Just copy the file
		compiledContent = originalContent
	}
	sha := sha1.New()
	sha.Write(compiledContent)
	hash := fmt.Sprintf("%x", sha.Sum(nil))
	compiledFileName = hash + compiledExt
	err = ioutil.WriteFile(path.Join(compiledAssetsFolder, compiledFileName), compiledContent, 0644)
	if err != nil {
		return
	}
	return
}

// Function for templates
func assetFile(fileName string) string {
	return "/" + assetFiles[fileName]
}

func allAssetPaths() []string {
	var paths []string
	for _, name := range assetFiles {
		paths = append(paths, "/"+name)
	}
	return paths
}

// Gets only called by registered paths
func serveAsset(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "public,max-age=31536000,immutable")
	http.ServeFile(w, r, path.Join(compiledAssetsFolder, r.URL.Path))
}
