package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
		Transport: httpcachetransport.NewHTTPCacheTransportNoBody(
			newHTTPTransport(),
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
	pls, err := a.extractPostLinks(posts)
	if err != nil {
		return nil, err
	}
	var out []*stringPair
	for _, pl := range pls {
		postURL := a.fullPostURL(pl.post)
		for _, link := range pl.links {
			out = append(out, &stringPair{postURL, link})
		}
	}
	return out, nil
}

type postLinks struct {
	post  *post
	links []string
}

func (a *goBlog) extractPostLinks(posts []*post) ([]*postLinks, error) {
	type res struct {
		pl  *postLinks
		err error
	}
	ch := make(chan res, len(posts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for _, p := range posts {
		wg.Add(1)
		sem <- struct{}{}
		go func(p *post) {
			defer wg.Done()
			defer func() { <-sem }()
			pr, pw := io.Pipe()
			go func() {
				a.postHTMLToWriter(pw, &postHTMLOptions{p: p, absolute: true})
				_ = pw.Close()
			}()
			links, err := allLinksFromHTML(pr, a.fullPostURL(p))
			_ = pr.CloseWithError(err)
			if err != nil {
				ch <- res{err: err}
				return
			}
			links = lo.Filter(links, func(s string, _ int) bool {
				return !a.isLocalURL(s)
			})
			ch <- res{pl: &postLinks{post: p, links: links}}
		}(p)
	}
	wg.Wait()
	close(ch)
	out := make([]*postLinks, 0, len(posts))
	for r := range ch {
		if r.err != nil {
			return nil, r.err
		}
		out = append(out, r.pl)
	}
	return out, nil
}

func successStatus(status int) bool {
	return status >= 200 && status < 400
}
