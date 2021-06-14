package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

func (a *goBlog) checkAllExternalLinks() {
	allPosts, err := a.db.getPosts(&postsRequestConfig{status: statusPublished})
	if err != nil {
		log.Println(err.Error())
		return
	}
	wg := new(sync.WaitGroup)
	linkChan := make(chan stringPair)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	responses := map[string]int{}
	rm := sync.RWMutex{}
	processFunc := func() {
		defer wg.Done()
		wg.Add(1)
		for postLinkPair := range linkChan {
			if strings.HasPrefix(postLinkPair.Second, a.cfg.Server.PublicAddress) {
				continue
			}
			rm.RLock()
			_, ok := responses[postLinkPair.Second]
			rm.RUnlock()
			if !ok {
				req, err := http.NewRequest(http.MethodGet, postLinkPair.Second, nil)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				// User-Agent from Tor
				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; rv:60.0) Gecko/20100101 Firefox/60.0")
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
				req.Header.Set("Accept-Language", "en-US,en;q=0.5")
				resp, err := client.Do(req)
				if err != nil {
					fmt.Println(postLinkPair.Second+" ("+postLinkPair.First+"):", err.Error())
					continue
				}
				status := resp.StatusCode
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				rm.Lock()
				responses[postLinkPair.Second] = status
				rm.Unlock()
			}
			rm.RLock()
			if response, ok := responses[postLinkPair.Second]; ok && !checkSuccessStatus(response) {
				fmt.Println(postLinkPair.Second+" ("+postLinkPair.First+"):", response)
			}
			rm.RUnlock()
		}
	}
	for i := 0; i < 20; i++ {
		go processFunc()
	}
	err = a.getExternalLinks(allPosts, linkChan)
	if err != nil {
		log.Println(err.Error())
		return
	}
	wg.Wait()
}

func checkSuccessStatus(status int) bool {
	return status >= 200 && status < 400
}

func (a *goBlog) getExternalLinks(posts []*post, linkChan chan<- stringPair) error {
	wg := new(sync.WaitGroup)
	for _, p := range posts {
		wg.Add(1)
		go func(p *post) {
			defer wg.Done()
			links, _ := allLinksFromHTMLString(string(a.absoluteHTML(p)), a.fullPostURL(p))
			for _, link := range links {
				linkChan <- stringPair{a.fullPostURL(p), link}
			}
		}(p)
	}
	wg.Wait()
	close(linkChan)
	return nil
}
