package main

import (
	"context"
	"strings"

	"github.com/carlmjohnson/requests"
)

func (ntfy *configNtfy) enabled() bool {
	if ntfy == nil || !ntfy.Enabled || ntfy.Topic == "" {
		return false
	}
	return true
}

func (a *goBlog) sendNtfy(cfg *configNtfy, msg string) error {
	if !cfg.enabled() {
		return nil
	}
	return requests.
		URL(cfg.Topic).
		Client(a.httpClient).
		UserAgent(appUserAgent).
		Post().
		BodyReader(strings.NewReader(msg)).
		Fetch(context.Background())
}
