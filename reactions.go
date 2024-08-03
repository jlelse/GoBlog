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

// Hardcoded for now
var allowedReactions = []string{
	"❤️", "👍", "🎉", "😂", "😱",
}
var allowedReactionsStr = strings.Join(allowedReactions, "")

func (a *goBlog) reactionsEnabled() bool {
	return a.cfg.Reactions != nil && a.cfg.Reactions.Enabled
}

const reactionsPostParam = "reactions"

func (a *goBlog) reactionsEnabledForPost(post *post) bool {
	return a.reactionsEnabled() && post != nil && post.firstParameter(reactionsPostParam) != "false"
}

func (a *goBlog) initReactions() {
	a.reactionsInit.Do(func() {
		if !a.reactionsEnabled() {
			return
		}
		a.reactionsCache = c.New[string, string](time.Minute, 100)
	})
}

func (a *goBlog) deleteReactionsCache(path string) {
	a.initReactions()
	if a.reactionsCache != nil {
		a.reactionsCache.Delete(path)
	}
}

func (a *goBlog) postReaction(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	reaction := r.FormValue("reaction")
	if path == "" || reaction == "" {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Save reaction
	err := a.saveReaction(reaction, path)
	if err != nil {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Return new values
	a.getReactions(w, r)
}

func (a *goBlog) saveReaction(reaction, path string) error {
	// Check if reaction is allowed
	if !lo.Contains(allowedReactions, reaction) {
		return errors.New("reaction not allowed")
	}
	// Init
	a.initReactions()
	// Delete from cache
	defer a.reactionsSfg.Forget(path)
	defer a.reactionsCache.Delete(path)
	// Insert reaction
	_, err := a.db.Exec("insert into reactions (path, reaction, count) values (?, ?, 1) on conflict (path, reaction) do update set count=count+1", path, reaction)
	return err
}

func (a *goBlog) getReactions(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	reactions, err := a.getReactionsFromDatabase(path)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(cacheControl, "no-store")
	w.Header().Set(contentType, contenttype.JSONUTF8)
	io.WriteString(w, reactions)
}

const reactionsQuery = "select json_group_object(reaction, count) as json_result from (" +
	"select reaction, count from reactions where path = ? and instr(?, reaction) > 0 " +
	"and path not in (select path from post_parameters where parameter=? and value=?) and count > 0)"

func (a *goBlog) getReactionsFromDatabase(path string) (string, error) {
	// Init
	a.initReactions()
	// Check cache
	if val, cached := a.reactionsCache.Get(path); cached {
		// Return from cache
		return val, nil
	}
	// Get reactions
	res, err, _ := a.reactionsSfg.Do(path, func() (string, error) {
		row, err := a.db.QueryRow(reactionsQuery, path, allowedReactionsStr, reactionsPostParam, "false")
		if err != nil {
			return "", err
		}
		var jsonResult string
		err = row.Scan(&jsonResult)
		if err != nil {
			return "", err
		}
		// Cache result
		a.reactionsCache.Set(path, jsonResult, reactionsCacheTTL, 1)
		return jsonResult, nil
	})
	return res, err
}
