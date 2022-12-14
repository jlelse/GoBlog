package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/carlmjohnson/requests"
	"github.com/samber/lo"
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
	topic := strings.TrimPrefix(cfg.Topic, "ntfy.sh/")
	server, _ := lo.Coalesce(cfg.Server, "https://ntfy.sh")
	builder := requests.
		URL(server + "/" + topic).
		Client(a.httpClient).
		Method(http.MethodPost).
		BodyReader(strings.NewReader(msg))
	if cfg.User != "" {
		builder.BasicAuth(cfg.User, cfg.Pass)
	}
	if cfg.Email != "" {
		builder.Header("X-Email", cfg.Email)
	}
	return builder.Fetch(context.Background())
}
