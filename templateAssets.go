package main

import (
	"crypto/sha1"
	"fmt"
	"github.com/bep/golibsass/libsass"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const assetsFolder = "templates/assets"
const compiledAssetsFolder = "tmp_assets"

var assetFiles map[string]string

func initTemplateAssets() error {
	err := os.RemoveAll(compiledAssetsFolder)
	err = os.MkdirAll(compiledAssetsFolder, 0755)
	if err != nil {
		return err
	}
	assetFiles = map[string]string{}
	err = filepath.Walk(assetsFolder, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			compiled, err := compileAssets(path)
			if err != nil {
				return err
			}
			if compiled != "" {
				assetFiles[path] = compiled
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
	case ".scss":
		transpiler, err := libsass.New(libsass.Options{OutputStyle: libsass.CompressedStyle})
		if err != nil {
			return "", err
		}
		result, err := transpiler.Execute(string(originalContent))
		if err != nil {
			return "", err
		}
		compiledContent, err = minifier.Bytes("text/css", []byte(result.CSS))
		if err != nil {
			return "", err
		}
		compiledExt = ".css"
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
	return appConfig.Server.PublicAddress + "/" + assetFiles[fileName]
}

func allAssetPaths() []string {
	var paths []string
	for _, name := range assetFiles {
		paths = append(paths, "/"+name)
	}
	return paths
}

// Gets only called by registered paths
func serveAsset(path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "public,max-age=31536000,immutable")
		http.ServeFile(w, r, compiledAssetsFolder+path)
	}
}
