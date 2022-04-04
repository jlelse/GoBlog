package main

import (
	"context"
	"net/http"
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
	topic := cfg.Topic
	if strings.HasPrefix(topic, "ntfy.sh/") { // Old configuration example
		topic = strings.TrimPrefix(topic, "ntfy.sh/")
	}
	server := "https://ntfy.sh"
	if cfg.Server != "" {
		server = cfg.Server
	}
	builder := requests.
		URL(server + "/" + topic).
		Client(a.httpClient).
		UserAgent(appUserAgent).
		Method(http.MethodPost).
		BodyReader(strings.NewReader(msg))
	if cfg.User != "" {
		builder.BasicAuth(cfg.User, cfg.Pass)
	}
	return builder.Fetch(context.Background())
}
