package main

// Copied from https://github.com/zerok/webmentiond/blob/main/pkg/webmention/verify.go and modified

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/zerok/webmentiond/pkg/webmention"
	"golang.org/x/net/html"
	"willnorris.com/go/microformats"
)

type wmVerifyOptions struct {
	MaxRedirects int
}

func wmVerify(mention *webmention.Mention) error {
	client := &http.Client{}
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 15 {
			return errors.New("too many redirects")
		}
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, mention.Source, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return wmVerifyReader(resp.Body, mention)
}

func wmVerifyReader(body io.Reader, mention *webmention.Mention) error {
	var tokenBuffer bytes.Buffer
	var mfBuffer bytes.Buffer
	sourceURL, err := url.Parse(mention.Source)
	if err != nil {
		return err
	}
	io.Copy(io.MultiWriter(&tokenBuffer, &mfBuffer), body)
	tokenizer := html.NewTokenizer(&tokenBuffer)
	mf := microformats.Parse(&mfBuffer, sourceURL)
	inTitle := false
	inAudio := false
	inVideo := false
	title := ""
	u, err := url.Parse(mention.Source)
	if err == nil {
		title = u.Hostname()
	}
	var contentOK bool
loop:
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.TextToken:
			if inTitle {
				title = strings.TrimSpace(string(tokenizer.Text()))
			}
		case html.EndTagToken:
			tagName, _ := tokenizer.TagName()
			switch string(tagName) {
			case "title":
				inTitle = false
			case "audio":
				inAudio = false
			case "video":
				inVideo = false
			}
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				break loop
			}
			return err
		case html.SelfClosingTagToken:
			fallthrough
		case html.StartTagToken:
			tagName, _ := tokenizer.TagName()
			switch string(tagName) {
			case "title":
				inTitle = true
			case "audio":
				inAudio = true
			case "video":
				inVideo = true
			case "source":
				if inVideo || inAudio {
					src := getAttr(tokenizer, "src")
					if src == mention.Target {
						mention.Title = title
						contentOK = true
						continue
					}
				}
			case "img":
				src := getAttr(tokenizer, "src")
				if src == mention.Target {
					mention.Title = title
					contentOK = true
					continue
				}
			case "a":
				href := getAttr(tokenizer, "href")
				if href == mention.Target {
					mention.Title = title
					contentOK = true
					continue
				}
			}

		}
	}
	if !contentOK {
		return fmt.Errorf("target not found in content")
	}
	mfFillMentionFromData(mention, mf)
	return nil
}

func mfFillMentionFromData(mention *webmention.Mention, mf *microformats.Data) {
	for _, i := range mf.Items {
		mfFillMention(mention, i)
	}
}

func mfFillMention(mention *webmention.Mention, mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
			if title, ok := name[0].(string); ok {
				mention.Title = title
			}
		}
		if commented, ok := mf.Properties["in-reply-to"]; ok && len(commented) > 0 {
			if commentedItem, ok := commented[0].(string); ok && commentedItem == mention.Target {
				mention.Type = "comment"
			}
		}
		if commented, ok := mf.Properties["like-of"]; ok && len(commented) > 0 {
			if commentedItem, ok := commented[0].(string); ok && commentedItem == mention.Target {
				mention.Type = "like"
			}
		}
		if contents, ok := mf.Properties["content"]; ok && len(contents) > 0 {
			if content, ok := contents[0].(map[string]interface{}); ok {
				if rawContentValue, ok := content["value"]; ok {
					if contentValue, ok := rawContentValue.(string); ok {
						mention.Content = contentValue
					}
				}
			}
		}
		if authors, ok := mf.Properties["author"]; ok && len(authors) > 0 {
			if author, ok := authors[0].(*microformats.Microformat); ok {
				if names, ok := author.Properties["name"]; ok && len(names) > 0 {
					if authorName, ok := names[0].(string); ok {
						mention.AuthorName = authorName
					}
				}
			}
		}
		return true
	} else if len(mf.Children) > 0 {
		for _, m := range mf.Children {
			if mfFillMention(mention, m) {
				return true
			}
		}
	}
	return false
}

func mfHasType(mf *microformats.Microformat, typ string) bool {
	for _, t := range mf.Type {
		if typ == t {
			return true
		}
	}
	return false
}

func getAttr(tokenizer *html.Tokenizer, attr string) string {
	var result string
	for {
		key, value, more := tokenizer.TagAttr()
		if string(key) == attr {
			result = string(value)
		}
		if !more {
			break
		}
	}
	return result
}
