package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_urlize(t *testing.T) {
	if res := urlize("äbc ef"); res != "bc-ef" {
		t.Errorf("Wrong result, got: %v", res)
	}
}

func Test_sortedStrings(t *testing.T) {
	input := []string{"a", "c", "b"}
	if res := sortedStrings(input); !reflect.DeepEqual(res, []string{"a", "b", "c"}) {
		t.Errorf("Wrong result, got: %v", res)
	}
}

func Test_generateRandomString(t *testing.T) {
	if l := len(generateRandomString(30)); l != 30 {
		t.Errorf("Wrong length: %v", l)
	}
}

func Test_isAllowedHost(t *testing.T) {
	req1 := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	req2 := httptest.NewRequest(http.MethodGet, "https://example.com:443", nil)
	req3 := httptest.NewRequest(http.MethodGet, "http://example.com:80", nil)

	if isAllowedHost(req1, "example.com") != true {
		t.Error("Wrong result")
	}

	if isAllowedHost(req1, "example.net") != false {
		t.Error("Wrong result")
	}

	if isAllowedHost(req2, "example.com") != true {
		t.Error("Wrong result")
	}

	if isAllowedHost(req3, "example.com") != true {
		t.Error("Wrong result")
	}
}

func Test_isAbsoluteURL(t *testing.T) {
	if isAbsoluteURL("http://example.com") != true {
		t.Error("Wrong result")
	}

	if isAbsoluteURL("https://example.com") != true {
		t.Error("Wrong result")
	}

	if isAbsoluteURL("/test") != false {
		t.Error("Wrong result")
	}
}

func Test_wordCount(t *testing.T) {
	assert.Equal(t, 3, wordCount("abc def abc"))
}

func Test_charCount(t *testing.T) {
	assert.Equal(t, 4, charCount("  t  e\n  s  t €.☺️"))
}

func Test_allLinksFromHTMLString(t *testing.T) {
	baseUrl := "https://example.net/post/abc"
	html := `<a href="relative1">Test</a><a href="relative1">Test</a><a href="/relative2">Test</a><a href="https://example.com">Test</a>`
	expected := []string{"https://example.net/post/relative1", "https://example.net/relative2", "https://example.com"}

	if result, err := allLinksFromHTMLString(html, baseUrl); err != nil {
		t.Errorf("Got error: %v", err)
	} else if !reflect.DeepEqual(result, expected) {
		t.Errorf("Wrong result, got: %v", result)
	}
}