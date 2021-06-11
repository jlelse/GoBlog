package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func Test_robotsTXT(t *testing.T) {
	testRecorder := httptest.NewRecorder()
	testRequest := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	servePrivateRobotsTXT(testRecorder, testRequest)

	testResult := testRecorder.Result()
	if sc := testResult.StatusCode; sc != 200 {
		t.Errorf("Wrong status code, got: %v", sc)
	}
	if rb, _ := io.ReadAll(testResult.Body); !reflect.DeepEqual(rb, []byte("User-agent: *\nDisallow: /")) {
		t.Errorf("Wrong response body, got: %v", rb)
	}

	app := &goBlog{
		cfg: &config{
			Server: &configServer{
				PublicAddress: "https://example.com",
			},
		},
	}

	testRecorder = httptest.NewRecorder()
	testRequest = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	app.serveRobotsTXT(testRecorder, testRequest)

	testResult = testRecorder.Result()
	if sc := testResult.StatusCode; sc != 200 {
		t.Errorf("Wrong status code, got: %v", sc)
	}
	if rb, _ := io.ReadAll(testResult.Body); !reflect.DeepEqual(rb, []byte("User-agent: *\nSitemap: https://example.com/sitemap.xml")) {
		t.Errorf("Wrong response body, got: %v", string(rb))
	}
}
