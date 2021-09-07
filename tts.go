package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"go.goblog.app/app/pkgs/mp3merge"
)

func (a *goBlog) createPostTTSAudio(p *post) error {
	// Get required values
	lang := a.cfg.Blogs[p.Blog].Lang
	if lang == "" {
		lang = "en"
	}
	text := a.renderMdTitle(p.Title()) + "\n\n" + cleanHTMLText(string(a.postHtml(p, false)))

	// Generate audio file
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	outputFileName := filepath.Join(tmpDir, "audio.mp3")
	err = a.createTTSAudio(lang, text, outputFileName)
	if err != nil {
		return err
	}

	// Save new audio file
	file, err := os.Open(outputFileName)
	if err != nil {
		return err
	}
	fileHash, err := getSHA256(file)
	if err != nil {
		return err
	}
	loc, err := a.saveMediaFile(fileHash+".mp3", file)
	if err != nil {
		return err
	}

	// Set post parameter
	if loc != "" {
		err = a.db.replacePostParam(p.Path, "tts", []string{loc})
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *goBlog) createTTSAudio(lang, text, outputFile string) error {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Split text
	textParts := []string{}
	var textPartBuilder strings.Builder
	textRunes := []rune(text)
	for i, r := range textRunes {
		textPartBuilder.WriteRune(r)
		newText := false
		if strings.ContainsRune(",.:!?)", r) && i+1 < len(textRunes) && unicode.IsSpace(textRunes[i+1]) {
			newText = true
		} else if r == '\n' {
			newText = true
		} else if textPartBuilder.Len() > 500 && unicode.IsSpace(r) {
			newText = true
		}
		if newText {
			textParts = append(textParts, textPartBuilder.String())
			textPartBuilder.Reset()
		}
	}
	textParts = append(textParts, textPartBuilder.String())

	// Start request for every text part
	allFiles := []string{}
	var wg sync.WaitGroup
	var ttsErr error
	ctx, cancel := context.WithCancel(context.Background())
	for _, s := range textParts {
		s := strings.TrimSpace(s)
		if s == "" {
			continue
		}
		fileName := filepath.Join(tmpDir, generateRandomString(10)+".mp3")
		allFiles = append(allFiles, fileName)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := a.downloadTTSAudio(ctx, lang, s, fileName)
			if err != nil && ttsErr == nil {
				ttsErr = err
				cancel()
			}
		}()
	}
	wg.Wait()
	cancel()
	if ttsErr != nil {
		return ttsErr
	}

	// Merge MP3s
	if err = mp3merge.MergeMP3(outputFile, allFiles); err != nil {
		return err
	}

	return nil
}

func (a *goBlog) downloadTTSAudio(ctx context.Context, lang, text, outputFile string) error {
	// Check parameters
	if lang == "" {
		return errors.New("language not provided")
	}
	if text == "" {
		return errors.New("empty text")
	}
	if outputFile == "" {
		return errors.New("output file not provided")
	}

	// Encode params
	ttsUrlVals := url.Values{}
	ttsUrlVals.Set("client", "tw-ob")
	ttsUrlVals.Set("tl", lang)
	ttsUrlVals.Set("q", strings.TrimSpace(text))

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://translate.google.com/translate_tts?"+ttsUrlVals.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; rv:60.0) Gecko/20100101 Firefox/60.0")

	// Do request
	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("TTS: got status: %s, text: %s", res.Status, text)
	}

	// Save response
	if err = os.MkdirAll(path.Dir(outputFile), os.ModePerm); err != nil {
		return err
	}
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err = io.Copy(out, res.Body); err != nil {
		return err
	}

	return nil
}
