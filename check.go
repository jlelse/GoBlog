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

func checkAllExternalLinks() {
	allPosts, err := getPosts(&postsRequestConfig{status: statusPublished})
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
	for i := 0; i < 20; i++ {
		go func() {
			defer wg.Done()
			wg.Add(1)
			for postLinkPair := range linkChan {
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
		}()
	}
	err = getExternalLinks(allPosts, linkChan)
	if err != nil {
		log.Println(err.Error())
		return
	}
	wg.Wait()
}

func checkSuccessStatus(status int) bool {
	return status >= 200 && status < 400
}

func getExternalLinks(posts []*post, linkChan chan<- stringPair) error {
	wg := new(sync.WaitGroup)
	for _, p := range posts {
		wg.Add(1)
		go func(p *post) {
			defer wg.Done()
			links, _ := allLinksFromHTML(strings.NewReader(string(p.absoluteHTML())), p.fullURL())
			for _, link := range links {
				if !strings.HasPrefix(link, appConfig.Server.PublicAddress) {
					linkChan <- stringPair{p.fullURL(), link}
				}
			}
		}(p)
	}
	wg.Wait()
	close(linkChan)
	return nil
}
