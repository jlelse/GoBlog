package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/samber/lo"
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

	// concurrency control
	maxGoroutines := 10
	sem := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup
	resultsCh := make(chan *checkresult, len(allLinks))

	for _, link := range allLinks {
		if done.Load() {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(lp *stringPair) {
			defer func() { <-sem; wg.Done() }()
			if done.Load() {
				return
			}
			res := &checkresult{in: lp.First, link: lp.Second}
			// Build request
			req, err := requests.URL(lp.Second).
				UserAgent("Mozilla/5.0 (Windows NT 6.1; rv:60.0) Gecko/20100101 Firefox/60.0").
				Accept("text/html").
				Header("Accept-Language", "en-US,en;q=0.5").
				Request(cancelContext)
			if err != nil {
				res.err = err
				resultsCh <- res
				return
			}
			// Do request
			resp, err := client.Do(req)
			if err != nil {
				res.err = err
				resultsCh <- res
				return
			}
			res.status = resp.StatusCode
			_ = resp.Body.Close()
			resultsCh <- res
		}(link)
	}

	wg.Wait()
	close(resultsCh)
	for r := range resultsCh {
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
	// We'll run these in parallel but collect results and errors
	type res struct {
		links []*stringPair
		err   error
	}
	ch := make(chan res, len(posts))
	var wg sync.WaitGroup
	for _, pst := range posts {
		wg.Add(1)
		go func(pst *post) {
			defer wg.Done()
			pr, pw := io.Pipe()
			go func() {
				a.postHtmlToWriter(pw, &postHtmlOptions{p: pst, absolute: true})
				_ = pw.Close()
			}()
			links, err := allLinksFromHTML(pr, a.fullPostURL(pst))
			_ = pr.CloseWithError(err)
			if err != nil {
				ch <- res{nil, err}
				return
			}
			// Remove internal links
			links = lo.Filter(links, func(i string, _ int) bool { return !strings.HasPrefix(i, a.cfg.Server.PublicAddress) })
			// Map to string pair
			ch <- res{lo.Map(links, func(s string, _ int) *stringPair { return &stringPair{a.fullPostURL(pst), s} }), nil}
		}(pst)
	}
	wg.Wait()
	close(ch)
	var all [][]*stringPair
	for r := range ch {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.links)
	}
	return lo.Flatten(all), nil
}

func successStatus(status int) bool {
	return status >= 200 && status < 400
}
