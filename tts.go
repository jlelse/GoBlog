package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"go.goblog.app/app/pkgs/contenttype"
)

const ttsParameter = "tts"

func (a *goBlog) initTTS() {
	if !a.ttsEnabled() {
		return
	}
	createOrUpdate := func(p *post) {
		// Automatically create audio for published section posts only
		if !p.isPublishedSectionPost() {
			return
		}
		// Check if there is already a tts audio file
		if p.firstParameter(ttsParameter) != "" {
			return
		}
		// Create TTS audio
		err := a.createPostTTSAudio(p)
		if err != nil {
			log.Printf("create post audio for %s failed: %v", p.Path, err)
		}
	}
	a.pPostHooks = append(a.pPostHooks, createOrUpdate)
	a.pUpdateHooks = append(a.pUpdateHooks, createOrUpdate)
	a.pDeleteHooks = append(a.pDeleteHooks, func(p *post) {
		// Try to delete the audio file
		_ = a.deletePostTTSAudio(p)
	})
}

func (a *goBlog) ttsEnabled() bool {
	tts := a.cfg.TTS
	// Requires media storage as well
	return tts != nil && tts.Enabled && tts.GoogleAPIKey != "" && a.mediaStorageEnabled()
}

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
	if loc == "" {
		return errors.New("no media location for tts audio")
	}

	if old := p.firstParameter(ttsParameter); old != "" && old != loc {
		// Already has tts audio, but with different location
		// Try to delete the old audio file
		_ = a.deletePostTTSAudio(p)
	}

	// Set post parameter
	err = a.db.replacePostParam(p.Path, ttsParameter, []string{loc})
	if err != nil {
		return err
	}

	// Purge cache
	a.cache.purge()

	return nil
}

// Tries to delete the tts audio file, but doesn't remove the post parameter
func (a *goBlog) deletePostTTSAudio(p *post) bool {
	// Check if post has tts audio
	audio := p.firstParameter(ttsParameter)
	if audio == "" {
		return false
	}
	// Get filename and check if file is from the configured media storage
	fileUrl, err := url.Parse(audio)
	if err != nil {
		// Failed to parse audio url
		log.Println("failed to parse audio url:", err)
		return false
	}
	fileName := path.Base(fileUrl.Path)
	if a.getFullAddress(a.mediaFileLocation(fileName)) != audio {
		// File is not from the configured media storage
		return false
	}
	// Try to delete the audio file
	err = a.deleteMediaFile(fileName)
	if err != nil {
		log.Println("failed to delete audio file:", err)
		return false
	}
	return true
}

func (a *goBlog) createTTSAudio(lang, text, outputFile string) error {
	// Check if Google Cloud TTS is enabled
	gctts := a.cfg.TTS
	if !gctts.Enabled || gctts.GoogleAPIKey == "" {
		return errors.New("missing config for Google Cloud TTS")
	}

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

	// Create request body
	body := map[string]interface{}{
		"audioConfig": map[string]interface{}{
			"audioEncoding": "MP3",
		},
		"input": map[string]interface{}{
			"text": text,
		},
		"voice": map[string]interface{}{
			"languageCode": lang,
		},
	}
	jb, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// Create request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://texttospeech.googleapis.com/v1beta1/text:synthesize?key="+gctts.GoogleAPIKey, bytes.NewReader(jb))
	if err != nil {
		return err
	}
	req.Header.Set(contentType, contenttype.JSON)
	req.Header.Set(userAgent, appUserAgent)

	// Do request
	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("got status: %s, text: %s", res.Status, text)
	}

	// Decode response
	var content map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&content); err != nil {
		return err
	}
	if encoded, ok := content["audioContent"]; ok {
		if encodedStr, ok := encoded.(string); ok {
			if audio, err := base64.StdEncoding.DecodeString(encodedStr); err == nil {
				return os.WriteFile(outputFile, audio, os.ModePerm)
			} else {
				return err
			}
		}
	}
	return errors.New("no audio content")
}
