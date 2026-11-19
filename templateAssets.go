package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	"github.com/alecthomas/chroma/v3/styles"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/minify"
)

const assetsFolder = "templates/assets"

type assetFile struct {
	contentType  string
	body         []byte
	sha256base64 string
}

func (a *goBlog) initTemplateAssets() error {
	a.assetFileNames = map[string]string{}
	a.assetFiles = map[string]*assetFile{}
	a.assetPathSet = map[string]bool{}
	if err := filepath.Walk(assetsFolder, func(path string, info os.FileInfo, _ error) error {
		if info.Mode().IsRegular() {
			// Open file
			file, err := os.Open(path) //nolint:gosec
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
	var minType string
	switch ext {
	case ".js":
		minType = contenttype.JS
	case ".css":
		minType = contenttype.CSS
	case ".xml":
		minType = contenttype.XML
	}
	var body []byte
	var err error
	if minType != "" {
		var minBuf bytes.Buffer
		if err = minify.Get().Minify(minType, &minBuf, read); err != nil {
			return err
		}
		body = minBuf.Bytes()
	} else {
		body, err = io.ReadAll(read)
		if err != nil {
			return err
		}
	}
	// File name and base64 hash
	sum := sha256.Sum256(body)
	compiledFileName := hex.EncodeToString(sum[:]) + ext
	// Save file
	a.assetFiles[compiledFileName] = &assetFile{
		contentType:  mime.TypeByExtension(ext),
		body:         body,
		sha256base64: base64.StdEncoding.EncodeToString(sum[:]),
	}
	// Save mapping of original file name to compiled file name
	a.assetFileNames[name] = compiledFileName
	// Track compiled asset paths for allowlist checks
	a.assetPathSet["/"+compiledFileName] = true
	// Signal that the assets changed
	a.assetVersion.Add(1)
	return nil
}

// Function for templates
func (a *goBlog) assetFileName(fileName string) string {
	return "/" + a.assetFileNames[fileName]
}

func (a *goBlog) assetFileHash(fileName string) string {
	if af, ok := a.assetFiles[a.assetFileNames[fileName]]; ok && af != nil {
		return "sha256-" + af.sha256base64
	}
	return ""
}

func (a *goBlog) allAssetPaths() []string {
	paths := make([]string, 0, len(a.assetFileNames))
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

// isAssetPath reports whether the given request path is a compiled asset file path.
func (a *goBlog) isAssetPath(p string) bool {
	return a.assetPathSet[p]
}

func (a *goBlog) initChromaCSS() error {
	chromaPath := "css/chroma.css"
	// Check if file already exists
	if _, ok := a.assetFiles[chromaPath]; ok {
		return nil
	}
	// Initialize the style
	chromaStyle, err := styles.Get(chromaStyleName).Builder().Build()
	if err != nil {
		return err
	}
	// Generate and minify CSS
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := chromahtml.New(chromahtml.ClassPrefix("c-")).WriteCSS(buf, chromaStyle); err != nil {
		return err
	}
	return a.compileAsset(chromaPath, buf)
}
