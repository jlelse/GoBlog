package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/carlmjohnson/requests"
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
	lang := a.cfg.Blogs[p.Blog].Lang
	if lang == "" {
		lang = "en"
	}

	// Build SSML
	var ssml strings.Builder
	ssml.WriteString("<speak>")
	ssml.WriteString(html.EscapeString(a.renderMdTitle(p.Title())))
	ssml.WriteString("<break time=\"1s\"/>")
	ssml.WriteString(html.EscapeString(cleanHTMLText(string(a.postHtml(p, false)))))
	ssml.WriteString("</speak>")

	// Generate audio
	var audioBuffer bytes.Buffer
	err := a.createTTSAudio(lang, ssml.String(), &audioBuffer)
	if err != nil {
		return err
	}

	// Save audio
	audioReader := bytes.NewReader(audioBuffer.Bytes())
	fileHash, err := getSHA256(audioReader)
	if err != nil {
		return err
	}
	loc, err := a.saveMediaFile(fileHash+".mp3", audioReader)
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

	// Check max length
	// TODO: Support longer texts by splitting into multiple requests
	// if len(ssml) > 5000 {
	//	return errors.New("text is too long")
	// }

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
