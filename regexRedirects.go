package main

import (
	"net/http"
	"regexp"
)

var regexRedirects []*regexRedirect

type regexRedirect struct {
	From *regexp.Regexp
	To   string
}

func initRedirects() error {
	for _, cr := range appConfig.PathRedirects {
		re, err := regexp.Compile(cr.From)
		if err != nil {
			return err
		}
		regexRedirects = append(regexRedirects, &regexRedirect{
			From: re,
			To:   cr.To,
		})
	}
	return nil
}

func checkRegexRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oldPath := r.URL.Path
		for _, re := range regexRedirects {
			newPath := re.From.ReplaceAllString(oldPath, re.To)
			if oldPath != newPath {
				r.URL.Path = newPath
				http.Redirect(w, r, r.URL.String(), http.StatusFound)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
