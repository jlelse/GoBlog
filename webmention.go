package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

type webmentionStatus string

const (
	webmentionStatusVerified webmentionStatus = "verified"
	webmentionStatusApproved webmentionStatus = "approved"

	webmentionPath = "/webmention"
)

type mention struct {
	ID      int
	Source  string
	Target  string
	Created int64
	Title   string
	Content string
	Author  string
	Status  webmentionStatus
}

func initWebmention() error {
	// Add hooks
	hookFunc := func(p *post) {
		if p.Status == statusPublished {
			_ = p.sendWebmentions()
		}
	}
	postHooks[postPostHook] = append(postHooks[postPostHook], hookFunc)
	postHooks[postUpdateHook] = append(postHooks[postUpdateHook], hookFunc)
	postHooks[postDeleteHook] = append(postHooks[postDeleteHook], hookFunc)
	// Start verifier
	return initWebmentionQueue()
}

func handleWebmention(w http.ResponseWriter, r *http.Request) {
	m, err := extractMention(r)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if !isAllowedHost(httptest.NewRequest(http.MethodGet, m.Target, nil), appConfig.Server.publicHostname) {
		serveError(w, r, "target not allowed", http.StatusBadRequest)
		return
	}
	if err = queueMention(m); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, "Webmention accepted")
}

func extractMention(r *http.Request) (*mention, error) {
	if !strings.Contains(r.Header.Get(contentType), contentTypeWWWForm) {
		return nil, errors.New("Unsupported Content-Type")
	}
	err := r.ParseForm()
	if err != nil {
		return nil, err
	}
	source := r.Form.Get("source")
	target := unescapedPath(r.Form.Get("target"))
	if source == "" || target == "" || !isAbsoluteURL(source) || !isAbsoluteURL(target) {
		return nil, errors.New("Invalid request")
	}
	return &mention{
		Source:  source,
		Target:  target,
		Created: time.Now().Unix(),
	}, nil
}

func webmentionExists(source, target string) bool {
	result := 0
	row, err := appDbQueryRow("select exists(select 1 from webmentions where source = ? and target = ?)", source, target)
	if err != nil {
		return false
	}
	if err = row.Scan(&result); err != nil {
		return false
	}
	return result == 1
}

func createWebmention(source, target string) (err error) {
	return queueMention(&mention{
		Source:  source,
		Target:  unescapedPath(target),
		Created: time.Now().Unix(),
	})
}

func deleteWebmention(id int) error {
	_, err := appDbExec("delete from webmentions where id = @id", sql.Named("id", id))
	return err
}

func approveWebmention(id int) error {
	_, err := appDbExec("update webmentions set status = ? where id = ?", webmentionStatusApproved, id)
	return err
}

type webmentionsRequestConfig struct {
	target        string
	status        webmentionStatus
	asc           bool
	offset, limit int
}

func buildWebmentionsQuery(config *webmentionsRequestConfig) (query string, args []interface{}) {
	args = []interface{}{}
	filter := ""
	if config != nil {
		if config.target != "" && config.status != "" {
			filter = "where target = @target and status = @status"
			args = append(args, sql.Named("target", unescapedPath(config.target)), sql.Named("status", config.status))
		} else if config.target != "" {
			filter = "where target = @target"
			args = append(args, sql.Named("target", config.target))
		} else if config.status != "" {
			filter = "where status = @status"
			args = append(args, sql.Named("status", config.status))
		}
	}
	order := "desc"
	if config.asc {
		order = "asc"
	}
	query = "select id, source, target, created, title, content, author, status from webmentions " + filter + " order by created " + order
	if config.limit != 0 || config.offset != 0 {
		query += " limit @limit offset @offset"
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return query, args
}

func getWebmentions(config *webmentionsRequestConfig) ([]*mention, error) {
	mentions := []*mention{}
	query, args := buildWebmentionsQuery(config)
	rows, err := appDbQuery(query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		m := &mention{}
		err = rows.Scan(&m.ID, &m.Source, &m.Target, &m.Created, &m.Title, &m.Content, &m.Author, &m.Status)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, m)
	}
	return mentions, nil
}

func countWebmentions(config *webmentionsRequestConfig) (count int, err error) {
	query, params := buildWebmentionsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := appDbQueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}
