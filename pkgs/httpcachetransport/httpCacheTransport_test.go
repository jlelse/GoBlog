package httpcachetransport

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/assert"
)

const fakeResponse = `HTTP/1.1 200 OK
Content-Type: text/html; charset=UTF-8
Date: Wed, 14 Dec 2022 10:34:03 GMT

<!doctype html>
<html>
</html>`

func TestHttpCacheTransport(t *testing.T) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters:        100,
		MaxCost:            10,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})

	counter := 0

	orig := requests.RoundTripFunc(func(req *http.Request) (res *http.Response, err error) {
		counter++
		return http.ReadResponse(bufio.NewReader(strings.NewReader(fakeResponse)), req)
	})

	client := &http.Client{
		Transport: NewHttpCacheTransport(orig, cache, time.Minute),
	}

	err := requests.URL("https://example.com/").Client(client).Fetch(context.Background())
	assert.NoError(t, err)

	err = requests.URL("https://example.com/").Client(client).Fetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 1, counter)
}
