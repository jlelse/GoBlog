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
			time.Sleep(5 * time.Second)
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
	if err := appDb.QueryRow("select exists(select 1 from webmentions where source = ? and target = ?)", source, target).Scan(&result); err != nil {
		return false
	}
	return result == 1
}

func verifyNextWebmention() error {
	m := &mention{}
	oldStatus := ""
	if err := appDb.QueryRow("select id, source, target, status from webmentions where (status = ? or status = ?) limit 1", webmentionStatusNew, webmentionStatusRenew).Scan(&m.ID, &m.Source, &m.Target, &oldStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
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
	startWritingToDb()
	defer finishWritingToDb()
	_, err := appDb.Exec("update webmentions set status = ?, title = ?, type = ?, content = ?, author = ? where id = ?", webmentionStatusVerified, wmm.Title, wmm.Type, wmm.Content, wmm.AuthorName, m.ID)
	if oldStatus == string(webmentionStatusNew) {
		sendNotification(fmt.Sprintf("New webmention from %s to %s", m.Source, m.Target))
	}
	return err
}

func createWebmention(source, target string) (err error) {
	if webmentionExists(source, target) {
		startWritingToDb()
		defer finishWritingToDb()
		_, err = appDb.Exec("update webmentions set status = ? where source = ? and target = ?", webmentionStatusRenew, source, target)
	} else {
		startWritingToDb()
		defer finishWritingToDb()
		_, err = appDb.Exec("insert into webmentions (source, target, created) values (?, ?, ?)", source, target, time.Now().Unix())
	}
	return err
}

func deleteWebmention(id int) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := appDb.Exec("delete from webmentions where id = ?", id)
	return err
}

func approveWebmention(id int) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := appDb.Exec("update webmentions set status = ? where id = ?", webmentionStatusApproved, id)
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
	filter := "where 1 = 1 "
	args := []interface{}{}
	if config != nil {
		if config.target != "" {
			filter += "and target = ? "
			args = append(args, config.target)
		}
		if config.status != "" {
			filter += "and status = ? "
			args = append(args, config.status)
		}
	}
	rows, err = appDb.Query("select id, source, target, created, title, content, author, type from webmentions "+filter+"order by created desc", args...)
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

// TODO: Integrate
func sendWebmentions(url string, prefixBlocks ...string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	discovered, err := webmention.DiscoverLinksFromReader(resp.Body, url, ".h-entry")
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	var filtered []string
	allowed := func(link string) bool {
		for _, block := range prefixBlocks {
			if strings.HasPrefix(link, block) {
				return false
			}
		}
		return true
	}
	for _, link := range discovered {
		if allowed(link) {
			filtered = append(filtered, link)
		}
	}
	client := webmention.New(nil)
	for _, link := range filtered {
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
