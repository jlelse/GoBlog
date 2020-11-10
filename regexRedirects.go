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
		regexRedirects = append(regexRedirects, &regexRedirect{
			From: re,
			To:   cr.To,
			Type: cr.Type,
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
				code := re.Type
				if code == 0 {
					code = http.StatusFound
				}
				http.Redirect(w, r, r.URL.String(), code)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
