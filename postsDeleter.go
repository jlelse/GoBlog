package main

import (
	"log"
	"time"

	"github.com/araddon/dateparse"
)

func (a *goBlog) initPostsDeleter() {
	a.hourlyHooks = append(a.hourlyHooks, func() {
		a.checkDeletedPosts()
	})
}

func (a *goBlog) checkDeletedPosts() {
	// Get all posts with `deleted` parameter and a deleted status
	postsToDelete, err := a.getPosts(&postsRequestConfig{
		status:    []postStatus{statusPublishedDeleted, statusDraftDeleted, statusScheduledDeleted},
		parameter: "deleted",
	})
	if err != nil {
		log.Println("Error getting deleted posts:", err)
		return
	}
	for _, post := range postsToDelete {
		// Check if post is deleted for more than 7 days
		if deleted, err := dateparse.ParseLocal(post.firstParameter("deleted")); err == nil && deleted.Add(time.Hour*24*7).Before(time.Now()) {
			if err := a.deletePost(post.Path); err != nil {
				log.Println("Error deleting post:", err)
			}
		}
	}
}
