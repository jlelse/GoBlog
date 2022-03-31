package main

import (
	"log"
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
		log.Println("Posts scheduler stopped")
	})
}

func (a *goBlog) checkScheduledPosts() {
	postsToPublish, err := a.getPosts(&postsRequestConfig{
		status:          "scheduled",
		publishedBefore: time.Now(),
	})
	if err != nil {
		log.Println("Error getting scheduled posts:", err)
		return
	}
	for _, post := range postsToPublish {
		post.Status = "published"
		err := a.replacePost(post, post.Path, statusScheduled)
		if err != nil {
			log.Println("Error publishing scheduled post:", err)
			continue
		}
		log.Println("Published scheduled post:", post.Path)
	}
}
