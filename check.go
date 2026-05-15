package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

// checkOptions controls behavior of the link checker.
type checkOptions struct {
	// ignore403 suppresses reporting of HTTP 403 responses (often false positives
	// caused by Cloudflare and similar bot protection).
	ignore403 bool
	// checkDNSBL enables querying public DNS-based blocklists for each unique
	// linked domain (Spamhaus DBL and SURBL multi).
	checkDNSBL bool
}

func (a *goBlog) checkAllExternalLinks(opts *checkOptions) error {
	posts, err := a.getPosts(&postsRequestConfig{
		status:             []postStatus{statusPublished},
		visibility:         []postVisibility{visibilityPublic, visibilityUnlisted},
		fetchWithoutParams: true,
	})
	if err != nil {
		return err
	}
	return a.checkLinks(opts, posts...)
}

func (a *goBlog) checkLinks(opts *checkOptions, posts ...*post) error {
	if opts == nil {
		opts = &checkOptions{}
	}
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
			req, err := requests.URL(lp.Second).
				UserAgent("Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0").
				Accept("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8").
				Header("Accept-Language", "en-US,en;q=0.5").
				Header("DNT", "1").
				Header("Upgrade-Insecure-Requests", "1").
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
			if opts.ignore403 && r.status == http.StatusForbidden {
				continue
			}
			fmt.Printf("%s in %s: %d (%s)\n", r.link, r.in, r.status, http.StatusText(r.status))
		}
	}
	fmt.Println("Finished link check")

	if opts.checkDNSBL {
		if done.Load() {
			return nil
		}
		a.checkDomainsDNSBL(cancelContext, allLinks, &done)
	}

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

var dnsblZones = []string{
	"dbl.spamhaus.org",
}

func (a *goBlog) checkDomainsDNSBL(ctx context.Context, links []*stringPair, done *atomic.Bool) {
	domainPosts := map[string][]string{}
	for _, lp := range links {
		d := dnsblHost(lp.Second)
		if d == "" {
			continue
		}
		posts := domainPosts[d]
		if !lo.Contains(posts, lp.First) {
			domainPosts[d] = append(posts, lp.First)
		}
	}
	if len(domainPosts) == 0 {
		return
	}
	fmt.Println("Checking", len(domainPosts), "domains against DNS blocklists:", strings.Join(dnsblZones, ", "))

	resolver := &net.Resolver{}

	type dnsblResult struct {
		domain, zone, codes string
		posts               []string
	}
	resultsCh := make(chan *dnsblResult, len(domainPosts)*len(dnsblZones))

	maxGoroutines := 10
	sem := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	for domain, posts := range domainPosts {
		if done.Load() {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(domain string, posts []string) {
			defer func() { <-sem; wg.Done() }()
			if done.Load() {
				return
			}
			for _, zone := range dnsblZones {
				if done.Load() {
					return
				}
				query := domain + "." + zone
				lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				ips, err := resolver.LookupIP(lookupCtx, "ip4", query)
				cancel()
				if err != nil || len(ips) == 0 {
					continue
				}
				codes := make([]string, 0, len(ips))
				skip := false
				for _, ip := range ips {
					ipStr := ip.String()
					if ipStr == "127.255.255.254" {
						skip = true
						break
					}
					codes = append(codes, ipStr)
				}
				if skip {
					continue
				}
				resultsCh <- &dnsblResult{domain: domain, zone: zone, codes: strings.Join(codes, ","), posts: posts}
			}
		}(domain, posts)
	}
	wg.Wait()
	close(resultsCh)
	listed := 0
	for r := range resultsCh {
		listed++
		fmt.Printf("DNSBL: %s listed on %s (%s) - linked from:\n", r.domain, r.zone, r.codes)
		for _, p := range r.posts {
			fmt.Printf("  - %s\n", p)
		}
	}
	fmt.Printf("Finished DNSBL check (%d listings)\n", listed)
}

func dnsblHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ""
	}
	return host
}
