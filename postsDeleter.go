package main

import (
	"time"

	"github.com/araddon/dateparse"
)

func (a *goBlog) initPostsDeleter() {
	a.hourlyHooks = append(a.hourlyHooks, func() {
		a.checkDeletedPosts()
	})
}

const deletedPostParam = "deleted"

func (a *goBlog) checkDeletedPosts() {
	// Get all posts with `deleted` parameter and a deleted status
	postsToDelete, err := a.getPosts(&postsRequestConfig{
		status:    []postStatus{statusPublishedDeleted, statusDraftDeleted, statusScheduledDeleted},
		parameter: deletedPostParam,
	})
	if err != nil {
		a.error("Error getting deleted posts", "err", err)
		return
	}
	for _, post := range postsToDelete {
		// Check if post is deleted for more than 7 days
		if deleted, err := dateparse.ParseLocal(post.firstParameter(deletedPostParam)); err == nil &&
			deleted.Add(time.Hour*24*7).Before(time.Now()) {
			if err := a.deletePost(post.Path); err != nil {
				a.error("Error deleting post", "err", err)
			}
		}
	}
}
