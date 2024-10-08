package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	"go.goblog.app/app/pkgs/bodylimit"
	cpkg "go.goblog.app/app/pkgs/cache"
	"go.goblog.app/app/pkgs/httpcachetransport"
)

func (a *goBlog) checkAllExternalLinks() error {
	posts, err := a.getPosts(&postsRequestConfig{
		status:             []postStatus{statusPublished},
		visibility:         []postVisibility{visibilityPublic, visibilityUnlisted},
		fetchWithoutParams: true,
	})
	if err != nil {
		return err
	}
	return a.checkLinks(posts...)
}

func (a *goBlog) checkLinks(posts ...*post) error {
	// Get all links
	allLinks, err := a.allLinksToCheck(posts...)
	if err != nil {
		return err
	}
	// Print some info
	fmt.Println("Checking", len(allLinks), "links")
	// Cancel context
	cancelContext, cancelFunc := context.WithCancel(context.Background())
	var done atomic.Bool
	a.shutdown.Add(func() {
		done.Store(true)
		cancelFunc()
		fmt.Println("Cancelled link check")
	})
	// Create HTTP client
	cache := cpkg.New[string, []byte](time.Minute, 5000)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: httpcachetransport.NewHttpCacheTransportNoBody(
			newHttpTransport(),
			cache, 60*time.Minute, 5*bodylimit.MB,
		),
	}
	// Process all links
	type checkresult struct {
		in, link string
		status   int
		err      error
	}
	p := pool.NewWithResults[*checkresult]().WithMaxGoroutines(10).WithContext(cancelContext)
	for _, link := range allLinks {
		link := link
		p.Go(func(ctx context.Context) (result *checkresult, _ error) {
			if done.Load() {
				return nil, nil
			}
			result = &checkresult{
				in:   link.First,
				link: link.Second,
			}
			// Build request
			req, err := requests.URL(link.Second).
				UserAgent("Mozilla/5.0 (Windows NT 6.1; rv:60.0) Gecko/20100101 Firefox/60.0").
				Accept("text/html").
				Header("Accept-Language", "en-US,en;q=0.5").
				Request(ctx)
			if err != nil {
				result.err = err
				return
			}
			// Do request
			resp, err := client.Do(req)
			if err != nil {
				result.err = err
				return
			}
			// Save status code
			result.status = resp.StatusCode
			// Close request
			_ = resp.Body.Close()
			return
		})
	}
	results, _ := p.Wait()
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.err != nil {
			fmt.Printf("%s in %s: %s\n", r.link, r.in, r.err.Error())
		} else if !successStatus(r.status) {
			fmt.Printf("%s in %s: %d (%s)\n", r.link, r.in, r.status, http.StatusText(r.status))
		}
	}
	fmt.Println("Finished link check")
	return nil
}

func (a *goBlog) allLinksToCheck(posts ...*post) ([]*stringPair, error) {
	p := pool.NewWithResults[[]*stringPair]().WithErrors()
	for _, post := range posts {
		post := post
		p.Go(func() ([]*stringPair, error) {
			pr, pw := io.Pipe()
			go func() {
				a.postHtmlToWriter(pw, &postHtmlOptions{p: post, absolute: true})
				_ = pw.Close()
			}()
			links, err := allLinksFromHTML(pr, a.fullPostURL(post))
			_ = pr.CloseWithError(err)
			if err != nil {
				return nil, err
			}
			// Remove internal links
			links = lo.Filter(links, func(i string, _ int) bool { return !strings.HasPrefix(i, a.cfg.Server.PublicAddress) })
			// Map to string pair
			return lo.Map(links, func(s string, _ int) *stringPair { return &stringPair{a.fullPostURL(post), s} }), nil
		})
	}
	results, err := p.Wait()
	return lo.Flatten(results), err
}

func successStatus(status int) bool {
	return status >= 200 && status < 400
}
