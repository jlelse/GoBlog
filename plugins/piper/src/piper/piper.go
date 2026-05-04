// Package piper provides a local Piper-based TTS plugin for GoBlog.
//
// It runs the piper binary as a subprocess, pipes its raw PCM output through
// ffmpeg to produce MP3 and writes the result to the provided writer.
package piper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	binary         string
	ffmpeg         string
	defaultVoice   string
	voices         map[string]string
	maxConcurrency int
	rateLock       sync.Mutex
	rateCache      map[string]int
	validated      bool
	validateOnce   sync.Once
	validateErr    error
	sem            chan struct{}
}

// GetPlugin returns the piper TTS plugin instance.
func GetPlugin() (plugintypes.SetConfig, plugintypes.SetApp, plugintypes.TTS) {
	p := &plugin{
		binary:         "piper",
		ffmpeg:         "ffmpeg",
		voices:         map[string]string{},
		rateCache:      map[string]int{},
		maxConcurrency: 1,
	}
	return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) { p.app = app }

func (p *plugin) SetConfig(config map[string]any) {
	if v, ok := config["binary"].(string); ok && v != "" {
		p.binary = v
	}
	if v, ok := config["ffmpegbinary"].(string); ok && v != "" {
		p.ffmpeg = v
	}
	if v, ok := config["defaultvoice"].(string); ok && v != "" {
		p.defaultVoice = v
	}
	if v, ok := config["voices"].(map[string]any); ok {
		for lang, path := range v {
			if s, ok := path.(string); ok && s != "" {
				p.voices[strings.TrimSpace(strings.ToLower(lang))] = s
			}
		}
	}
	if v, ok := config["maxconcurrency"].(int); ok && v > 0 {
		p.maxConcurrency = v
	}
	p.sem = make(chan struct{}, p.maxConcurrency)
	log.Printf("piper plugin configured with binary: %s ffmpeg: %s defaultvoice: %s voices: %v maxconcurrency: %d", p.binary, p.ffmpeg, p.defaultVoice, p.voices, p.maxConcurrency)
	p.validated = false
}

func (p *plugin) validateBinaries() error {
	p.validateOnce.Do(func() {
		for _, bin := range []string{p.binary, p.ffmpeg} {
			if _, err := exec.LookPath(bin); err != nil {
				p.validateErr = fmt.Errorf("piper: required binary %q not found in PATH: %w", bin, err)
				return
			}
		}
		p.validated = true
	})
	return p.validateErr
}

func (p *plugin) voiceForLang(lang string) string {
	l := strings.TrimSpace(strings.ToLower(lang))
	if l == "" {
		return p.defaultVoice
	}
	if v, ok := p.voices[l]; ok {
		return v
	}
	return p.defaultVoice
}

func (p *plugin) sampleRate(voice string) int {
	p.rateLock.Lock()
	defer p.rateLock.Unlock()
	if r, ok := p.rateCache[voice]; ok {
		return r
	}
	rate := 22050
	jsonPath := voice + ".json"
	f, err := os.Open(jsonPath)
	if err != nil {
		p.rateCache[voice] = rate
		return rate
	}
	defer f.Close()
	var cfg struct {
		Audio struct {
			SampleRate int `json:"sample_rate"`
		} `json:"audio"`
	}
	if err := json.NewDecoder(f).Decode(&cfg); err == nil && cfg.Audio.SampleRate > 0 {
		rate = cfg.Audio.SampleRate
	}
	p.rateCache[voice] = rate
	return rate
}

func (p *plugin) SynthesizeSpeech(lang, text string, w io.Writer) error {
	if text == "" {
		return errors.New("piper: empty text")
	}
	if w == nil {
		return errors.New("piper: writer not provided")
	}
	if err := p.validateBinaries(); err != nil {
		return err
	}
	voice := p.voiceForLang(lang)
	if voice == "" {
		return fmt.Errorf("piper: no voice configured for language %q and no defaultvoice set", lang)
	}
	rate := p.sampleRate(voice)

	p.sem <- struct{}{}
	buf, err := p.synthesize(voice, rate, text)
	<-p.sem
	if err != nil {
		return err
	}
	_, err = io.Copy(w, buf)
	return err
}

func (p *plugin) synthesize(voice string, rate int, text string) (*bytes.Buffer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	piperCmd := exec.CommandContext(ctx, p.binary, "--model", voice, "--output_raw")
	piperCmd.Stdin = strings.NewReader(text)
	piperStderr := builderpool.Get()
	defer builderpool.Put(piperStderr)
	piperCmd.Stderr = piperStderr
	piperOut, err := piperCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("piper: stdout pipe: %w", err)
	}

	ffmpegCmd := exec.CommandContext(ctx, p.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-threads", "1",
		"-f", "s16le", "-ar", strconv.Itoa(rate), "-ac", "1",
		"-i", "-",
		"-f", "mp3", "-",
	)
	ffmpegCmd.Stdin = piperOut
	ffmpegStderr := builderpool.Get()
	defer builderpool.Put(ffmpegStderr)
	ffmpegCmd.Stderr = ffmpegStderr
	buf := bytes.NewBuffer(nil)
	ffmpegCmd.Stdout = buf

	if err := piperCmd.Start(); err != nil {
		return nil, fmt.Errorf("piper: start: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		_ = piperCmd.Process.Kill()
		_ = piperCmd.Wait()
		return nil, fmt.Errorf("piper: ffmpeg start: %w", err)
	}
	ffmpegErr := ffmpegCmd.Wait()
	_ = piperCmd.Wait()
	if ffmpegErr != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("piper: synthesis timed out after 1 minute")
		}
		return nil, fmt.Errorf("piper: ffmpeg: %w (%s)", ffmpegErr, strings.TrimSpace(ffmpegStderr.String()))
	}
	return buf, nil
}
