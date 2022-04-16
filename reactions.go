package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
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
	// Insert reaction
	_, err := a.db.exec("insert into reactions (path, reaction, count) values (?, ?, 1) on conflict (path, reaction) do update set count=count+1", path, reaction)
	return err
}

func (a *goBlog) getReactions(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	reactions, err := a.getReactionsFromDatabase(path)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err = json.NewEncoder(buf).Encode(reactions)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_ = a.min.Get().Minify(contenttype.JSON, w, buf)
}

func (a *goBlog) getReactionsFromDatabase(path string) (map[string]int, error) {
	sqlBuf := bufferpool.Get()
	defer bufferpool.Put(sqlBuf)
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
	rows, err := a.db.query(sqlBuf.String(), sqlArgs...)
	if err != nil {
		return nil, err
	}
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
	return reactions, nil
}
