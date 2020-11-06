package main

import (
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"
)

func urlize(str string) string {
	newStr := ""
	for _, c := range strings.Split(strings.ToLower(str), "") {
		if c >= "a" && c <= "z" || c >= "A" && c <= "Z" || c >= "0" && c <= "9" {
			newStr += c
		} else if c == " " {
			newStr += "-"
		}
	}
	return newStr
}

func sortedStrings(s []string) []string {
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(s[i]) < strings.ToLower(s[j])
	})
	return s
}

func generateRandomString(chars int) string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, chars)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func isAllowedHost(r *http.Request, hosts ...string) bool {
	if r.URL == nil {
		return false
	}
	rh := r.URL.Host
	switch r.URL.Scheme {
	case "http":
		rh = strings.TrimSuffix(rh, ":80")
	case "https":
		rh = strings.TrimSuffix(rh, ":443")
	}
	for _, host := range hosts {
		if rh == host {
			return true
		}
	}
	return false
}
