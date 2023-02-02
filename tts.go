package main

import (
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

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/mp3merge"
	"golang.org/x/sync/errgroup"
)

const ttsParameter = "tts"

func (a *goBlog) initTTS() {
	if !a.ttsEnabled() {
		return
	}
	createOrUpdate := func(p *post) {
		// Automatically create audio for published section posts only
		if !p.isPublicPublishedSectionPost() {
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
		if a.deletePostTTSAudio(p) {
			log.Println("deleted tts audio for", p.Path)
		}
	})
}

func (a *goBlog) ttsEnabled() bool {
	tts := a.cfg.TTS
	// Requires media storage as well
	return tts != nil && tts.Enabled && tts.GoogleAPIKey != "" && a.mediaStorageEnabled()
}

func (a *goBlog) createPostTTSAudio(p *post) error {
	// Get required values
	lang := defaultIfEmpty(a.getBlogFromPost(p).Lang, "en")

	// Create TTS text parts
	parts := []string{}
	// Add title if available
	if title := p.Title(); title != "" {
		parts = append(parts, a.renderMdTitle(title))
	}
	// Add body split into paragraphs because of 5000 character limit
	phr, phw := io.Pipe()
	go func() {
		a.postHtmlToWriter(phw, &postHtmlOptions{p: p})
		_ = phw.Close()
	}()
	postHtmlText, err := htmlTextFromReader(phr)
	_ = phr.CloseWithError(err)
	if err != nil {
		return err
	}
	parts = append(parts, strings.Split(postHtmlText, "\n\n")...)

	// Create TTS audio for each part
	partReaders := []io.Reader{}
	partWriters := []*io.PipeWriter{}
	var g errgroup.Group
	for _, part := range parts {
		part := part
		pr, pw := io.Pipe()
		partReaders = append(partReaders, pr)
		partWriters = append(partWriters, pw)
		g.Go(func() error {
			// Build SSML
			ssml := "<speak>" + html.EscapeString(part) + "<break time=\"500ms\"/></speak>"
			// Create TTS audio
			err := a.createTTSAudio(lang, ssml, pw)
			_ = pw.CloseWithError(err)
			return err
		})
	}

	// Merge parts together (needs buffer because the hash is needed before the file can be uploaded)
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hash := sha256.New()
	err = mp3merge.MergeMP3(io.MultiWriter(buf, hash), partReaders...)
	for _, pw := range partWriters {
		_ = pw.CloseWithError(err)
	}
	if err != nil {
		return err
	}

	// Check if other errors appeared
	if err = g.Wait(); err != nil {
		return err
	}

	// Save audio
	loc, err := a.saveMediaFile(fmt.Sprintf("%x.mp3", hash.Sum(nil)), buf)
	if err != nil {
		return err
	}
	if loc == "" {
		return errors.New("no media location for tts audio")
	}

	// Check existing tts parameter
	if old := p.firstParameter(ttsParameter); old != "" && old != loc {
		// Already has tts audio, but with different location
		// Try to delete the old audio file
		if a.deletePostTTSAudio(p) {
			log.Println("deleted old tts audio for", p.Path)
		}
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
	body := map[string]any{
		"audioConfig": map[string]any{
			"audioEncoding": "MP3",
		},
		"input": map[string]any{
			"ssml": ssml,
		},
		"voice": map[string]any{
			"languageCode": lang,
		},
	}

	// Do request
	var response map[string]any
	err := requests.
		URL("https://texttospeech.googleapis.com/v1beta1/text:synthesize").
		Param("key", gctts.GoogleAPIKey).
		Client(a.httpClient).
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
			_, err := io.Copy(w, base64.NewDecoder(base64.StdEncoding, strings.NewReader(encodedStr)))
			return err
		}
	}
	return errors.New("no audio content")
}
