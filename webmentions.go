package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	wmd "github.com/zerok/webmentiond/pkg/webmention"
	"willnorris.com/go/webmention"
)

type webmentionStatus string

const (
	webmentionStatusNew      webmentionStatus = "new"
	webmentionStatusRenew    webmentionStatus = "renew"
	webmentionStatusVerified webmentionStatus = "verified"
	webmentionStatusApproved webmentionStatus = "approved"
)

type mention struct {
	ID      int
	Source  string
	Target  string
	Created int64
	Title   string
	Content string
	Author  string
	Type    string
}

func initWebmention() {
	startWebmentionVerifier()
}

func startWebmentionVerifier() {
	go func() {
		for {
			verifyNextWebmention()
			time.Sleep(30 * time.Second)
		}
	}()
}

func handleWebmention(w http.ResponseWriter, r *http.Request) {
	m, err := wmd.ExtractMention(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !isAllowedHost(httptest.NewRequest(http.MethodGet, m.Target, nil), r.URL.Host, appConfig.Server.Domain) {
		http.Error(w, "target not allowed", http.StatusBadRequest)
		return
	}
	if err = createWebmention(m.Source, m.Target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprint(w, "Webmention accepted")
}

func webmentionAdmin(w http.ResponseWriter, r *http.Request) {
	verified, err := getWebmentions(&webmentionsRequestConfig{
		status: webmentionStatusVerified,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	approved, err := getWebmentions(&webmentionsRequestConfig{
		status: webmentionStatusApproved,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "webmentionadmin", &renderData{
		Data: map[string][]*mention{
			"Verified": verified,
			"Approved": approved,
		},
	})
}

func webmentionAdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = deleteWebmention(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/webmention/admin", http.StatusFound)
	return
}

func webmentionAdminApprove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = approveWebmention(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/webmention/admin", http.StatusFound)
	return
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

func verifyNextWebmention() error {
	m := &mention{}
	oldStatus := ""
	row, err := appDbQueryRow("select id, source, target, status from webmentions where (status = ? or status = ?) limit 1", webmentionStatusNew, webmentionStatusRenew)
	if err != nil {
		return err
	}
	if err := row.Scan(&m.ID, &m.Source, &m.Target, &oldStatus); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	wmm := &wmd.Mention{
		Source: m.Source,
		Target: m.Target,
	}
	if err := wmd.Verify(context.Background(), wmm, func(c *wmd.VerifyOptions) {
		c.MaxRedirects = 15
	}); err != nil {
		// Invalid
		return deleteWebmention(m.ID)
	}
	if len(wmm.Content) > 500 {
		wmm.Content = wmm.Content[0:497] + "…"
	}
	newStatus := webmentionStatusVerified
	if strings.HasPrefix(wmm.Source, appConfig.Server.PublicAddress) {
		// Approve if it's server-intern
		newStatus = webmentionStatusApproved
	}
	_, err = appDbExec("update webmentions set status = ?, title = ?, type = ?, content = ?, author = ? where id = ?", newStatus, wmm.Title, wmm.Type, wmm.Content, wmm.AuthorName, m.ID)
	if oldStatus == string(webmentionStatusNew) {
		sendNotification(fmt.Sprintf("New webmention from %s to %s", m.Source, m.Target))
	}
	return err
}

func createWebmention(source, target string) (err error) {
	if webmentionExists(source, target) {
		_, err = appDbExec("update webmentions set status = ? where source = ? and target = ?", webmentionStatusRenew, source, target)
	} else {
		_, err = appDbExec("insert into webmentions (source, target, created) values (?, ?, ?)", source, target, time.Now().Unix())
	}
	return err
}

func deleteWebmention(id int) error {
	_, err := appDbExec("delete from webmentions where id = ?", id)
	return err
}

func approveWebmention(id int) error {
	_, err := appDbExec("update webmentions set status = ? where id = ?", webmentionStatusApproved, id)
	return err
}

type webmentionsRequestConfig struct {
	target string
	status webmentionStatus
}

func getWebmentions(config *webmentionsRequestConfig) ([]*mention, error) {
	mentions := []*mention{}
	var rows *sql.Rows
	var err error
	args := []interface{}{}
	filter := ""
	if config != nil {
		if config.target != "" && config.status != "" {
			filter = "where target = @target and status = @status"
			args = append(args, sql.Named("target", config.target), sql.Named("status", config.status))
		} else if config.target != "" {
			filter = "where target = @target"
			args = append(args, sql.Named("target", config.target))
		} else if config.status != "" {
			filter = "where status = @status"
			args = append(args, sql.Named("status", config.status))
		}
	}
	rows, err = appDbQuery("select id, source, target, created, title, content, author, type from webmentions "+filter+" order by created desc", args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		m := &mention{}
		err = rows.Scan(&m.ID, &m.Source, &m.Target, &m.Created, &m.Title, &m.Content, &m.Author, &m.Type)
		if err != nil {
			return nil, err
		}
		mentions = append(mentions, m)
	}
	return mentions, nil
}

func (p *post) sendWebmentions() error {
	url := appConfig.Server.PublicAddress + p.Path
	recorder := httptest.NewRecorder()
	// Render basic post data
	render(recorder, "postbasic", &renderData{
		blogString: p.Blog,
		Data:       p,
	})
	discovered, err := webmention.DiscoverLinksFromReader(recorder.Result().Body, url, ".h-entry")
	if err != nil {
		return err
	}
	client := webmention.New(nil)
	for _, link := range discovered {
		if strings.HasPrefix(link, appConfig.Server.PublicAddress) {
			// Save mention directly
			createWebmention(url, link)
			continue
		}
		endpoint, err := client.DiscoverEndpoint(link)
		if err != nil || len(endpoint) < 1 {
			continue
		}
		_, err = client.SendWebmention(endpoint, url, link)
		if err != nil {
			log.Println("Sending webmention to " + link + " failed")
			continue
		}
		log.Println("Sent webmention to " + link)
	}
	return nil
}
