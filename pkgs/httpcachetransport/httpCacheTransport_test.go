package httpcachetransport

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/bodylimit"
	cpkg "go.goblog.app/app/pkgs/cache"
)

const fakeResponse = `HTTP/1.1 200 OK
Content-Type: text/html; charset=UTF-8
Date: Wed, 14 Dec 2022 10:34:03 GMT

<!doctype html>
<html>
</html>`

func TestHttpCacheTransport(t *testing.T) {
	cache := cpkg.New[string, []byte](time.Minute, 10)

	counter := 0

	orig := requests.RoundTripFunc(func(req *http.Request) (res *http.Response, err error) {
		counter++
		return http.ReadResponse(bufio.NewReader(strings.NewReader(fakeResponse)), req)
	})

	client := &http.Client{
		Transport: NewHttpCacheTransport(orig, cache, time.Minute, bodylimit.KB),
	}

	err := requests.URL("https://example.com/").Client(client).Fetch(context.Background())
	assert.NoError(t, err)

	err = requests.URL("https://example.com/").Client(client).Fetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 1, counter)
}
