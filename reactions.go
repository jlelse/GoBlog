package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
	c "go.goblog.app/app/pkgs/cache"
	"go.goblog.app/app/pkgs/contenttype"
)

const reactionsCacheTTL = 6 * time.Hour

func (a *goBlog) getAllowedReactions(blog string) []string {
	bc, ok := a.cfg.Blogs[blog]
	if !ok {
		return []string{"❤️", "👍", "🎉", "😂", "😱"}
	}
	if len(bc.allowedReactions) == 0 {
		return []string{"❤️", "👍", "🎉", "😂", "😱"}
	}
	return bc.allowedReactions
}

func (a *goBlog) getAllowedReactionsStr(blog string) string {
	return strings.Join(a.getAllowedReactions(blog), "")
}

func (a *goBlog) reactionsEnabled(blog string) bool {
	bc, ok := a.cfg.Blogs[blog]
	return ok && bc.reactionsEnabled
}

func (a *goBlog) anyReactionsEnabled() bool {
	for _, bc := range a.cfg.Blogs {
		if bc.reactionsEnabled {
			return true
		}
	}
	return false
}

const reactionsPostParam = "reactions"

func (a *goBlog) reactionsEnabledForPost(p *post) bool {
	return p != nil && a.reactionsEnabled(p.Blog) && p.firstParameter(reactionsPostParam) != "false"
}

func (a *goBlog) initReactions() {
	a.reactionsInit.Do(func() {
		// Just initialize the cache, enablement check should be done when using it
		a.reactionsCache = c.New[string, string](time.Minute, 100)
	})
}

func (a *goBlog) purgeReactionsCache() {
	a.initReactions()
	if a.reactionsCache != nil {
		a.reactionsCache.Clear()
	}
}

func (a *goBlog) deleteReactionsCache(path string) {
	a.initReactions()
	if a.reactionsCache != nil {
		a.reactionsCache.Delete(path)
	}
}

func (a *goBlog) postReaction(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")         //nolint:gosec
	reaction := r.FormValue("reaction") //nolint:gosec
	if path == "" || reaction == "" {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Get post to check blog
	p, err := a.getPost(path)
	if err != nil {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Save reaction
	err = a.saveReaction(reaction, p)
	if err != nil {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Return new values
	a.getReactions(w, r)
}

func (a *goBlog) saveReaction(reaction string, p *post) error {
	// Check if reaction is allowed
	if !lo.Contains(a.getAllowedReactions(p.Blog), reaction) {
		return errors.New("reaction not allowed")
	}
	// Init
	a.initReactions()
	// Delete from cache
	defer a.reactionsSfg.Forget(p.Path)
	defer a.reactionsCache.Delete(p.Path)
	// Insert reaction
	_, err := a.db.Exec("insert into reactions (path, reaction, count) values (?, ?, 1) on conflict (path, reaction) do update set count=count+1", p.Path, reaction)
	return err
}

func (a *goBlog) getReactions(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path") //nolint:gosec
	p, err := a.getPost(path)
	if err != nil {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	reactions, err := a.getReactionsFromDatabase(p)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(cacheControl, "no-store")
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_, _ = io.WriteString(w, reactions) //nolint:gosec
}

const reactionsQuery = "select json_group_object(reaction, count) as json_result from (" +
	"select reaction, count from reactions where path = ? and instr(?, reaction) > 0 " +
	"and path not in (select path from post_parameters where parameter=? and value=?) and count > 0)"

func (a *goBlog) getReactionsFromDatabase(p *post) (string, error) {
	// Init
	a.initReactions()
	// Check cache
	if val, cached := a.reactionsCache.Get(p.Path); cached {
		// Return from cache
		return val, nil
	}
	// Get reactions
	res, err, _ := a.reactionsSfg.Do(p.Path, func() (string, error) {
		row, err := a.db.QueryRow(reactionsQuery, p.Path, a.getAllowedReactionsStr(p.Blog), reactionsPostParam, "false")
		if err != nil {
			return "", err
		}
		var jsonResult string
		err = row.Scan(&jsonResult)
		if err != nil {
			return "", err
		}
		// Cache result
		a.reactionsCache.Set(p.Path, jsonResult, reactionsCacheTTL, 1)
		return jsonResult, nil
	})
	return res, err
}
