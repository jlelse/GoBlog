package main

import (
	"time"
)

func (a *goBlog) startPostsScheduler() {
	ticker := time.NewTicker(30 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				a.checkScheduledPosts()
			}
		}
	}()
	a.shutdown.Add(func() {
		ticker.Stop()
		done <- struct{}{}
		a.info("Posts scheduler stopped")
	})
}

func (a *goBlog) checkScheduledPosts() {
	postsToPublish, err := a.getPosts(&postsRequestConfig{
		status:          []postStatus{statusScheduled},
		publishedBefore: time.Now(),
	})
	if err != nil {
		a.error("Error getting scheduled posts", "err", err)
		return
	}
	for _, post := range postsToPublish {
		post.Status = statusPublished
		err := a.replacePost(post, post.Path, statusScheduled, post.Visibility, true)
		if err != nil {
			a.error("Error publishing scheduled post", "err", err)
			continue
		}
		a.info("Published scheduled post", "path", post.Path)
	}
}
