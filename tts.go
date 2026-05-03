package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
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
			a.error("create post audio failed", "path", p.Path, "err", err)
		}
	}
	a.pPostHooks = append(a.pPostHooks, createOrUpdate)
	a.pUpdateHooks = append(a.pUpdateHooks, createOrUpdate)
	a.pUndeleteHooks = append(a.pUndeleteHooks, createOrUpdate)
	a.pDeleteHooks = append(a.pDeleteHooks, func(p *post) {
		// Try to delete the audio file
		if a.deletePostTTSAudio(p) {
			a.info("deleted tts audio", "path", p.Path)
		}
	})
}

func (a *goBlog) ttsEnabled() bool {
	tts := a.cfg.TTS
	if tts == nil || !tts.Enabled || !a.mediaStorageEnabled() {
		return false
	}
	if tts.MistralAPIKey != "" && tts.MistralVoice != "" {
		return true
	}
	if tts.GoogleAPIKey != "" {
		return true
	}
	return false
}

func (a *goBlog) createPostTTSAudio(p *post) error {
	// Get required values
	lang := cmp.Or(a.getBlogFromPost(p).Lang, "en")

	// Create TTS text parts
	parts := []string{}
	// Add title if available
	if title := p.Title(); title != "" {
		parts = append(parts, a.renderMdTitle(title))
	}
	// Add body split into paragraphs because of 5000 character limit
	phr, phw := io.Pipe()
	go func() {
		a.postHTMLToWriter(phw, &postHTMLOptions{p: p})
		_ = phw.Close()
	}()
	postHTMLText, err := htmlTextFromReader(phr)
	_ = phr.CloseWithError(err)
	if err != nil {
		return err
	}
	parts = append(parts, strings.Split(postHTMLText, "\n\n")...)

	// Create TTS audio for each part
	partReaders := []io.Reader{}
	partWriters := []*io.PipeWriter{}
	var g errgroup.Group
	for _, part := range parts {
		pr, pw := io.Pipe()
		partReaders = append(partReaders, pr)
		partWriters = append(partWriters, pw)
		g.Go(func() error {
			// Create TTS audio
			err := a.createTTSAudio(lang, part, pw)
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
			a.info("deleted old tts audio", "path", p.Path)
		}
	}

	// Set post parameter
	err = a.db.replacePostParam(p.Path, ttsParameter, []string{loc})
	if err != nil {
		return err
	}

	// Purge cache
	a.purgeCache()

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
	fileURL, err := url.Parse(audio)
	if err != nil {
		// Failed to parse audio url
		a.error("failed to parse audio url", "err", err, "audio", audio)
		return false
	}
	fileName := path.Base(fileURL.Path)
	if a.getFullAddress(a.mediaFileLocation(fileName)) != audio {
		// File is not from the configured media storage
		return false
	}
	// Try to delete the audio file
	err = a.deleteMediaFile(fileName)
	if err != nil {
		a.error("failed to delete audio file", "err", err, "file", fileName)
		return false
	}
	return true
}

func (a *goBlog) createTTSAudio(lang, text string, w io.Writer) error {
	tts := a.cfg.TTS
	if tts == nil || !tts.Enabled {
		return errors.New("tts not enabled")
	}

	// Check parameters
	if lang == "" {
		return errors.New("language not provided")
	}
	if text == "" {
		return errors.New("empty text")
	}
	if w == nil {
		return errors.New("writer not provided")
	}

	// Mistral takes precedence if configured
	if tts.MistralAPIKey != "" && tts.MistralVoice != "" {
		return a.createMistralTTSAudio(text, w)
	}
	if tts.GoogleAPIKey != "" {
		return a.createGoogleTTSAudio(lang, text, w)
	}
	return errors.New("missing config for TTS provider")
}

func (a *goBlog) createGoogleTTSAudio(lang, text string, w io.Writer) error {
	gctts := a.cfg.TTS

	// Build SSML
	ssml := "<speak>" + html.EscapeString(text) + "<break time=\"500ms\"/></speak>"

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
		URL("https://texttospeech.googleapis.com/v1/text:synthesize").
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

func (a *goBlog) createMistralTTSAudio(text string, w io.Writer) error {
	mtts := a.cfg.TTS

	// Create request body. Mistral requires both model and voice.
	body := map[string]any{
		"model":           cmp.Or(mtts.MistralModel, "voxtral-mini-tts-latest"),
		"voice":           mtts.MistralVoice,
		"input":           text,
		"response_format": "mp3",
	}

	// Do request
	var response map[string]any
	var errBody string
	err := requests.
		URL("https://api.mistral.ai/v1/audio/speech").
		Header("Authorization", "Bearer "+mtts.MistralAPIKey).
		Client(a.httpClient).
		Method(http.MethodPost).
		BodyJSON(body).
		AddValidator(func(res *http.Response) error {
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				b, _ := io.ReadAll(res.Body)
				errBody = string(b)
				return fmt.Errorf("unexpected status: %d", res.StatusCode)
			}
			return nil
		}).
		ToJSON(&response).
		Fetch(context.Background())
	if err != nil {
		if errBody != "" {
			return fmt.Errorf("tts request failed: %w: %s", err, errBody)
		}
		return errors.New("tts request failed: " + err.Error())
	}

	// Decode response
	if encoded, ok := response["audio_data"]; ok {
		if encodedStr, ok := encoded.(string); ok {
			_, err := io.Copy(w, base64.NewDecoder(base64.StdEncoding, strings.NewReader(encodedStr)))
			return err
		}
	}
	return errors.New("no audio content")
}
