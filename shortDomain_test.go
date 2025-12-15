package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectShortDomain(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	req := httptest.NewRequest(http.MethodGet, "http://short.example.com/short/path?query=1", nil)
	rr := httptest.NewRecorder()

	app.redirectShortDomain(rr, req)

	res := rr.Result()
	require.Equal(t, http.StatusMovedPermanently, res.StatusCode)
	assert.Equal(t, app.cfg.Server.getFullAddress("/short/path?query=1"), res.Header.Get("Location"))
}
