package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/mp3merge"
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
	a.pUndeleteHooks = append(a.pUndeleteHooks, createOrUpdate)
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
	lang := defaultIfEmpty(a.cfg.Blogs[p.Blog].Lang, "en")

	// Create TTS text parts
	parts := []string{}
	// Add title if available
	if title := p.Title(); title != "" {
		parts = append(parts, a.renderMdTitle(title))
	}
	// Add body split into paragraphs because of 5000 character limit
	parts = append(parts, strings.Split(htmlText(a.postHtml(p, false)), "\n\n")...)

	// Create TTS audio for each part
	partsBuffers := make([]io.Reader, len(parts))
	var errs []error
	var lock sync.Mutex
	var wg sync.WaitGroup
	for i, part := range parts {
		// Increase wait group
		wg.Add(1)
		go func(i int, part string) {
			// Build SSML
			ssml := "<speak>" + html.EscapeString(part) + "<break time=\"500ms\"/></speak>"
			// Create TTS audio
			var audioBuffer bytes.Buffer
			err := a.createTTSAudio(lang, ssml, &audioBuffer)
			if err != nil {
				lock.Lock()
				errs = append(errs, err)
				lock.Unlock()
				return
			}
			// Append buffer to partsBuffers
			lock.Lock()
			partsBuffers[i] = &audioBuffer
			lock.Unlock()
			// Decrease wait group
			wg.Done()
		}(i, part)
	}

	// Wait for all parts to be created
	wg.Wait()

	// Check if any errors occurred
	if len(errs) > 0 {
		return errs[0]
	}

	// Merge partsBuffers into final buffer
	final := new(bytes.Buffer)
	hash := sha256.New()
	if err := mp3merge.MergeMP3(io.MultiWriter(final, hash), partsBuffers...); err != nil {
		return err
	}

	// Save audio
	loc, err := a.saveMediaFile(fmt.Sprintf("%x.mp3", hash.Sum(nil)), final)
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

func (a *goBlog) createTTSAudio(lang, ssml string, w io.Writer) error {
	// Check if Google Cloud TTS is enabled
	gctts := a.cfg.TTS
	if !gctts.Enabled || gctts.GoogleAPIKey == "" {
		return errors.New("missing config for Google Cloud TTS")
	}

	// Check parameters
	if lang == "" {
		return errors.New("language not provided")
	}
	if ssml == "" {
		return errors.New("empty text")
	}
	if w == nil {
		return errors.New("writer not provided")
	}

	// Create request body
	body := map[string]interface{}{
		"audioConfig": map[string]interface{}{
			"audioEncoding": "MP3",
		},
		"input": map[string]interface{}{
			"ssml": ssml,
		},
		"voice": map[string]interface{}{
			"languageCode": lang,
		},
	}

	// Do request
	var response map[string]interface{}
	err := requests.
		URL("https://texttospeech.googleapis.com/v1beta1/text:synthesize").
		Param("key", gctts.GoogleAPIKey).
		Client(a.httpClient).
		UserAgent(appUserAgent).
		Method(http.MethodPost).
		BodyJSON(body).
		ToJSON(&response).
		Fetch(context.Background())
	if err != nil {
		return errors.New("tts request failed: " + err.Error())
	}

	// Decode response
	if encoded, ok := response["audioContent"]; ok {
		if encodedStr, ok := encoded.(string); ok {
			if audio, err := base64.StdEncoding.DecodeString(encodedStr); err == nil {
				_, err := w.Write(audio)
				return err
			} else {
				return err
			}
		}
	}
	return errors.New("no audio content")
}
