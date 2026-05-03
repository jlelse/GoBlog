package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ttsEnabled(t *testing.T) {
	newTestApp := func() *goBlog {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.mediaStorage = &localMediaStorage{path: filepath.Join(t.TempDir(), "media")}
		app.mediaStorageInit.Do(func() {})
		return app
	}

	t.Run("nil config", func(t *testing.T) {
		app := newTestApp()
		assert.False(t, app.ttsEnabled())
	})

	t.Run("disabled", func(t *testing.T) {
		app := newTestApp()
		app.cfg.TTS = &configTTS{Enabled: false, GoogleAPIKey: "abc"}
		assert.False(t, app.ttsEnabled())
	})

	t.Run("enabled but no api key", func(t *testing.T) {
		app := newTestApp()
		app.cfg.TTS = &configTTS{Enabled: true}
		assert.False(t, app.ttsEnabled())
	})

	t.Run("enabled with google key", func(t *testing.T) {
		app := newTestApp()
		app.cfg.TTS = &configTTS{Enabled: true, GoogleAPIKey: "abc"}
		assert.True(t, app.ttsEnabled())
	})

	t.Run("enabled with mistral key but no voice", func(t *testing.T) {
		app := newTestApp()
		app.cfg.TTS = &configTTS{Enabled: true, MistralAPIKey: "abc"}
		assert.False(t, app.ttsEnabled())
	})

	t.Run("enabled with mistral key and voice", func(t *testing.T) {
		app := newTestApp()
		app.cfg.TTS = &configTTS{Enabled: true, MistralAPIKey: "abc", MistralVoice: "v"}
		assert.True(t, app.ttsEnabled())
	})

	t.Run("enabled but no media storage", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.mediaStorageInit.Do(func() {})
		app.cfg.TTS = &configTTS{Enabled: true, MistralAPIKey: "abc", MistralVoice: "v"}
		assert.False(t, app.ttsEnabled())
	})
}

func Test_createTTSAudio_validation(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}

	t.Run("nil tts config", func(t *testing.T) {
		app.cfg.TTS = nil
		err := app.createTTSAudio("en", "hi", io.Discard)
		require.Error(t, err)
	})

	app.cfg.TTS = &configTTS{Enabled: true, GoogleAPIKey: "abc"}

	t.Run("missing language", func(t *testing.T) {
		err := app.createTTSAudio("", "hi", io.Discard)
		assert.EqualError(t, err, "language not provided")
	})
	t.Run("empty text", func(t *testing.T) {
		err := app.createTTSAudio("en", "", io.Discard)
		assert.EqualError(t, err, "empty text")
	})
	t.Run("nil writer", func(t *testing.T) {
		err := app.createTTSAudio("en", "hi", nil)
		assert.EqualError(t, err, "writer not provided")
	})
	t.Run("missing provider", func(t *testing.T) {
		app.cfg.TTS = &configTTS{Enabled: true}
		err := app.createTTSAudio("en", "hi", io.Discard)
		assert.EqualError(t, err, "missing config for TTS provider")
	})
}

func Test_createTTSAudio_google(t *testing.T) {
	const audio = "fake-google-mp3-bytes"
	encoded := base64.StdEncoding.EncodeToString([]byte(audio))

	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/text:synthesize", r.URL.Path)
		assert.Equal(t, "googlekey", r.URL.Query().Get("key"))
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		voice, _ := body["voice"].(map[string]any)
		assert.Equal(t, "de", voice["languageCode"])
		input, _ := body["input"].(map[string]any)
		ssml, _ := input["ssml"].(string)
		assert.True(t, strings.HasPrefix(ssml, "<speak>"))
		assert.Contains(t, ssml, "Hello &amp; world")
		assert.Contains(t, ssml, "<break time=\"500ms\"/>")

		_ = json.NewEncoder(w).Encode(map[string]any{"audioContent": encoded})
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{Enabled: true, GoogleAPIKey: "googlekey"}

	var out bytes.Buffer
	require.NoError(t, app.createTTSAudio("de", "Hello & world", &out))
	assert.Equal(t, audio, out.String())
	require.NotNil(t, fc.req)
	assert.Equal(t, "texttospeech.googleapis.com", fc.req.URL.Host)
}

func Test_createTTSAudio_mistral(t *testing.T) {
	const audio = "fake-mistral-mp3-bytes"
	encoded := base64.StdEncoding.EncodeToString([]byte(audio))

	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/audio/speech", r.URL.Path)
		assert.Equal(t, "Bearer mistralkey", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "voxtral-custom", body["model"])
		assert.Equal(t, "Hello world", body["input"])
		assert.Equal(t, "voice-xyz", body["voice"])
		assert.Equal(t, "mp3", body["response_format"])

		_ = json.NewEncoder(w).Encode(map[string]any{"audio_data": encoded})
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{
		Enabled:       true,
		MistralAPIKey: "mistralkey",
		MistralModel:  "voxtral-custom",
		MistralVoice:  "voice-xyz",
	}

	var out bytes.Buffer
	require.NoError(t, app.createTTSAudio("en", "Hello world", &out))
	assert.Equal(t, audio, out.String())
	require.NotNil(t, fc.req)
	assert.Equal(t, "api.mistral.ai", fc.req.URL.Host)
}

func Test_createTTSAudio_mistralDefaults(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("x"))

	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// Default model is used when not configured
		assert.Equal(t, "voxtral-mini-tts-latest", body["model"])
		// Voice is always sent
		assert.Equal(t, "v", body["voice"])

		_ = json.NewEncoder(w).Encode(map[string]any{"audio_data": encoded})
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{Enabled: true, MistralAPIKey: "mistralkey", MistralVoice: "v"}

	var out bytes.Buffer
	require.NoError(t, app.createTTSAudio("en", "Hi", &out))
	assert.Equal(t, "x", out.String())
}

func Test_createTTSAudio_mistralPrecedence(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("ok"))

	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Must hit Mistral, not Google
		assert.Equal(t, "api.mistral.ai", r.Host)
		_ = json.NewEncoder(w).Encode(map[string]any{"audio_data": encoded})
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{
		Enabled:       true,
		GoogleAPIKey:  "googlekey",
		MistralAPIKey: "mistralkey",
		MistralVoice:  "v",
	}

	var out bytes.Buffer
	require.NoError(t, app.createTTSAudio("en", "Hi", &out))
	assert.Equal(t, "api.mistral.ai", fc.req.URL.Host)
}

func Test_createTTSAudio_mistralFallsBackToGoogleWithoutVoice(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("ok"))

	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mistral key without voice should fall back to Google
		assert.Equal(t, "texttospeech.googleapis.com", r.Host)
		_ = json.NewEncoder(w).Encode(map[string]any{"audioContent": encoded})
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{
		Enabled:       true,
		GoogleAPIKey:  "googlekey",
		MistralAPIKey: "mistralkey",
	}

	var out bytes.Buffer
	require.NoError(t, app.createTTSAudio("en", "Hi", &out))
	assert.Equal(t, "texttospeech.googleapis.com", fc.req.URL.Host)
}

func Test_createTTSAudio_mistralErrorBody(t *testing.T) {
	fc := newFakeHttpClient()
	fc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid voice"}`))
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.TTS = &configTTS{Enabled: true, MistralAPIKey: "mistralkey", MistralVoice: "v"}

	err := app.createTTSAudio("en", "Hi", io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `{"error":"invalid voice"}`)
}
