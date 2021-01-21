package main

import (
	"net/http"
	"regexp"
)

var regexRedirects []*regexRedirect

type regexRedirect struct {
	From *regexp.Regexp
	To   string
	Type int
}

func initRegexRedirects() error {
	for _, cr := range appConfig.PathRedirects {
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
		regexRedirects = append(regexRedirects, r)
	}
	return nil
}

func checkRegexRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, re := range regexRedirects {
			if newPath := re.From.ReplaceAllString(r.URL.Path, re.To); r.URL.Path != newPath {
				r.URL.Path = newPath
				http.Redirect(w, r, r.URL.String(), re.Type)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
