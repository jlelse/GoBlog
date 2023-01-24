package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/contenttype"
)

type webmentionStatus string

const (
	webmentionStatusVerified webmentionStatus = "verified"
	webmentionStatusApproved webmentionStatus = "approved"

	webmentionPath = "/webmention"
)

type mention struct {
	ID          int
	Source      string
	NewSource   string
	Target      string
	NewTarget   string
	Url         string
	Created     int64
	Title       string
	Content     string
	Author      string
	Status      webmentionStatus
	Submentions []*mention
}

func (a *goBlog) initWebmention() {
	// Add hooks
	hookFunc := func(p *post) {
		_ = a.sendWebmentions(p)
	}
	a.pPostHooks = append(a.pPostHooks, hookFunc)
	a.pUpdateHooks = append(a.pUpdateHooks, hookFunc)
	a.pDeleteHooks = append(a.pDeleteHooks, hookFunc)
	a.pUndeleteHooks = append(a.pUndeleteHooks, hookFunc)
	// Start verifier
	a.initWebmentionQueue()
}

func (a *goBlog) handleWebmention(w http.ResponseWriter, r *http.Request) {
	m, err := a.extractMention(r)
	if err != nil {
		a.debug("Error extracting webmention:", err.Error())
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	hasShortPrefix := a.cfg.Server.ShortPublicAddress != "" && strings.HasPrefix(m.Target, a.cfg.Server.ShortPublicAddress)
	hasLongPrefix := strings.HasPrefix(m.Target, a.cfg.Server.PublicAddress)
	if !hasShortPrefix && !hasLongPrefix {
		a.debug("Webmention target not allowed:", m.Target)
		a.serveError(w, r, "target not allowed", http.StatusBadRequest)
		return
	}
	if m.Target == m.Source {
		a.debug("Webmention target and source are the same:", m.Target)
		a.serveError(w, r, "target and source are the same", http.StatusBadRequest)
		return
	}
	if err = a.queueMention(m); err != nil {
		a.debug("Failed to queue webmention", err.Error())
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, "Webmention accepted")
	a.debug("Accepted webmention:", m.Source, m.Target)
}

func (a *goBlog) extractMention(r *http.Request) (*mention, error) {
	if ct := r.Header.Get(contentType); !strings.Contains(ct, contenttype.WWWForm) {
		a.debug("New webmention request with wrong content type:", ct)
		return nil, errors.New("unsupported Content-Type")
	}
	err := r.ParseForm()
	if err != nil {
		return nil, err
	}
	source := r.Form.Get("source")
	target := r.Form.Get("target")
	if source == "" || target == "" || !isAbsoluteURL(source) || !isAbsoluteURL(target) {
		a.debug("Invalid webmention request, source:", source, "target:", target)
		return nil, errors.New("invalid request")
	}
	return &mention{
		Source:  source,
		Target:  target,
		Created: time.Now().Unix(),
	}, nil
}

func (db *database) webmentionExists(m *mention) bool {
	result := 0
	row, err := db.QueryRow(
		`
		select exists(
			select 1
			from webmentions
			where
				lowerunescaped(source) in (lowerunescaped(@source), lowerunescaped(@newsource))
				and lowerunescaped(target) in (lowerunescaped(@target), lowerunescaped(@newtarget))
		)
		`,
		sql.Named("source", m.Source), sql.Named("newsource", defaultIfEmpty(m.NewSource, m.Source)),
		sql.Named("target", m.Target), sql.Named("newtarget", defaultIfEmpty(m.NewTarget, m.Target)),
	)
	if err != nil {
		return false
	}
	if err = row.Scan(&result); err != nil {
		return false
	}
	return result == 1
}

func (a *goBlog) createWebmention(source, target string) (err error) {
	return a.queueMention(&mention{
		Source:  source,
		Target:  target,
		Created: time.Now().Unix(),
	})
}

func (db *database) insertWebmention(m *mention, status webmentionStatus) error {
	_, err := db.Exec(
		`
		insert into webmentions (source, target, url, created, status, title, content, author) 
		values (@source, lowerunescaped(@target), @url, @created, @status, @title, @content, @author)
		`,
		sql.Named("source", m.Source),
		sql.Named("target", m.Target),
		sql.Named("url", m.Url),
		sql.Named("created", m.Created),
		sql.Named("status", status),
		sql.Named("title", m.Title),
		sql.Named("content", m.Content),
		sql.Named("author", m.Author),
	)
	return err
}

func (db *database) updateWebmention(m *mention, newStatus webmentionStatus) error {
	_, err := db.Exec(`
			update webmentions
			set 
				source = @newsource,
				target = lowerunescaped(@newtarget),
				url = @url,
				status = @status,
				title = @title,
				content = @content,
				author = @author
			where
				lowerunescaped(source) in (lowerunescaped(@source), lowerunescaped(@newsource2))
				and lowerunescaped(target) in (lowerunescaped(@target), lowerunescaped(@newtarget2))
			`,
		sql.Named("newsource", defaultIfEmpty(m.NewSource, m.Source)),
		sql.Named("newtarget", defaultIfEmpty(m.NewTarget, m.Target)),
		sql.Named("url", m.Url),
		sql.Named("status", newStatus),
		sql.Named("title", m.Title),
		sql.Named("content", m.Content),
		sql.Named("author", m.Author),
		sql.Named("source", m.Source),
		sql.Named("newsource2", defaultIfEmpty(m.NewSource, m.Source)),
		sql.Named("target", m.Target),
		sql.Named("newtarget2", defaultIfEmpty(m.NewTarget, m.Target)),
	)
	return err
}

func (db *database) deleteWebmentionId(id int) error {
	_, err := db.Exec("delete from webmentions where id = @id", sql.Named("id", id))
	return err
}

func (db *database) deleteWebmentionUUrl(uUrl string) error {
	_, err := db.Exec("delete from webmentions where url = @url", sql.Named("url", uUrl))
	return err
}

func (db *database) deleteWebmention(m *mention) error {
	_, err := db.Exec(
		"delete from webmentions where lowerunescaped(source) in (lowerunescaped(@source), lowerunescaped(@newsource)) and lowerunescaped(target) in (lowerunescaped(@target), lowerunescaped(@newtarget))",
		sql.Named("source", m.Source),
		sql.Named("newsource", defaultIfEmpty(m.NewSource, m.Source)),
		sql.Named("target", m.Target),
		sql.Named("newtarget", defaultIfEmpty(m.NewTarget, m.Target)),
	)
	return err
}

func (db *database) approveWebmentionId(id int) error {
	_, err := db.Exec("update webmentions set status = ? where id = ?", webmentionStatusApproved, id)
	return err
}

func (a *goBlog) reverifyWebmentionId(id int) error {
	m, err := a.db.getWebmentions(&webmentionsRequestConfig{
		id:    id,
		limit: 1,
	})
	if err != nil {
		return err
	}
	if len(m) > 0 {
		err = a.queueMention(m[0])
	}
	return err
}

type webmentionsRequestConfig struct {
	target        string
	status        webmentionStatus
	sourcelike    string
	id            int
	asc           bool
	offset, limit int
	submentions   bool
}

func buildWebmentionsQuery(config *webmentionsRequestConfig) (query string, args []any) {
	queryBuilder := builderpool.Get()
	defer builderpool.Put(queryBuilder)
	queryBuilder.WriteString("select id, source, target, url, created, title, content, author, status from webmentions ")
	if config != nil {
		queryBuilder.WriteString("where 1")
		if config.target != "" {
			queryBuilder.WriteString(" and lowerunescaped(target) = lowerunescaped(@target)")
			args = append(args, sql.Named("target", config.target))
		}
		if config.status != "" {
			queryBuilder.WriteString(" and status = @status")
			args = append(args, sql.Named("status", config.status))
		}
		if config.sourcelike != "" {
			queryBuilder.WriteString(" and lowerunescaped(source) like ('%' || lowerunescaped(@sourcelike) || '%')")
			args = append(args, sql.Named("sourcelike", config.sourcelike))
		}
		if config.id != 0 {
			queryBuilder.WriteString(" and id = @id")
			args = append(args, sql.Named("id", config.id))
		}
	}
	queryBuilder.WriteString(" order by created ")
	if config.asc {
		queryBuilder.WriteString("asc")
	} else {
		queryBuilder.WriteString("desc")
	}
	if config.limit != 0 || config.offset != 0 {
		queryBuilder.WriteString(" limit @limit offset @offset")
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return queryBuilder.String(), args
}

func (db *database) getWebmentions(config *webmentionsRequestConfig) ([]*mention, error) {
	mentions := []*mention{}
	query, args := buildWebmentionsQuery(config)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		m := &mention{}
		err = rows.Scan(&m.ID, &m.Source, &m.Target, &m.Url, &m.Created, &m.Title, &m.Content, &m.Author, &m.Status)
		if err != nil {
			return nil, err
		}
		if m.Url == "" {
			m.Url = m.Source
		}
		if config.submentions {
			m.Submentions, err = db.getWebmentions(&webmentionsRequestConfig{
				target:      m.Source,
				submentions: false, // prevent infinite recursion
				asc:         config.asc,
				status:      config.status,
			})
			if err != nil {
				return nil, err
			}
		}
		mentions = append(mentions, m)
	}
	return mentions, nil
}

func (db *database) getWebmentionsByAddress(address string) []*mention {
	if address == "" {
		return nil
	}
	mentions, _ := db.getWebmentions(&webmentionsRequestConfig{
		target:      address,
		status:      webmentionStatusApproved,
		asc:         true,
		submentions: true,
	})
	return mentions
}

func (db *database) countWebmentions(config *webmentionsRequestConfig) (count int, err error) {
	query, params := buildWebmentionsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := db.QueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}
