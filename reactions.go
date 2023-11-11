package main

import (
	"errors"
	"net/http"

	"github.com/dgraph-io/ristretto"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/builderpool"
)

// Hardcoded for now
var allowedReactions = []string{
	"❤️",
	"👍",
	"🎉",
	"😂",
	"😱",
}

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
		a.reactionsCache, _ = ristretto.NewCache(&ristretto.Config{
			NumCounters:        1000,
			MaxCost:            100, // Cache reactions for 100 posts
			BufferItems:        64,
			IgnoreInternalCost: true,
		})
	})
}

func (a *goBlog) deleteReactionsCache(path string) {
	a.initReactions()
	if a.reactionsCache != nil {
		a.reactionsCache.Del(path)
	}
}

func (a *goBlog) postReaction(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	reaction := r.FormValue("reaction")
	if path == "" || reaction == "" {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	err := a.saveReaction(reaction, path)
	if err != nil {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
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
	defer a.reactionsCache.Del(path)
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
	a.respondWithMinifiedJson(w, reactions)
}

func (a *goBlog) getReactionsFromDatabase(path string) (map[string]int, error) {
	// Init
	a.initReactions()
	// Check cache
	if val, cached := a.reactionsCache.Get(path); cached {
		// Return from cache
		return val.(map[string]int), nil
	}
	// Get reactions
	res, err, _ := a.reactionsSfg.Do(path, func() (any, error) {
		// Build query
		sqlBuf := builderpool.Get()
		defer builderpool.Put(sqlBuf)
		sqlArgs := []any{}
		sqlBuf.WriteString("select reaction, count from reactions where path=? and reaction in (")
		sqlArgs = append(sqlArgs, path)
		for i, reaction := range allowedReactions {
			if i > 0 {
				sqlBuf.WriteString(",")
			}
			sqlBuf.WriteString("?")
			sqlArgs = append(sqlArgs, reaction)
		}
		sqlBuf.WriteString(") and path not in (select path from post_parameters where parameter=? and value=?)")
		sqlArgs = append(sqlArgs, reactionsPostParam, "false")
		// Execute query
		rows, err := a.db.Query(sqlBuf.String(), sqlArgs...)
		if err != nil {
			return nil, err
		}
		// Build result
		defer rows.Close()
		reactions := map[string]int{}
		for rows.Next() {
			var reaction string
			var count int
			err = rows.Scan(&reaction, &count)
			if err != nil {
				return nil, err
			}
			reactions[reaction] = count
		}
		// Cache result
		a.reactionsCache.Set(path, reactions, 1)
		return reactions, nil
	})
	if err != nil || res == nil {
		return nil, err
	}
	return res.(map[string]int), nil
}
