package main

import (
	"net/http"
	"regexp"
)

type regexRedirect struct {
	From *regexp.Regexp
	To   string
	Type int
}

func (a *goBlog) initRegexRedirects() error {
	for _, cr := range a.cfg.PathRedirects {
		re, err := regexp.Compile(cr.From)
		if err != nil {
			return err
		}
		r := &regexRedirect{
			From: re,
			To:   cr.To,
			Type: cr.Type,
		}
		if r.Type == 0 {
			r.Type = http.StatusFound
		}
		a.regexRedirects = append(a.regexRedirects, r)
	}
	return nil
}

func (a *goBlog) checkRegexRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, re := range a.regexRedirects {
			if newPath := re.From.ReplaceAllString(r.URL.Path, re.To); r.URL.Path != newPath {
				r.URL.Path = newPath
				http.Redirect(w, r, r.URL.String(), re.Type)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
