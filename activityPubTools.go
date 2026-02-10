package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/carlmjohnson/requests"
	ap "go.goblog.app/app/pkgs/activitypub"
	"go.goblog.app/app/pkgs/bodylimit"
)

type webfingerLink struct {
	Rel      string `json:"rel"`
	Type     string `json:"type,omitempty"`
	Href     string `json:"href,omitempty"`
	Template string `json:"template,omitempty"`
}
type webfingerResponse struct {
	Links []webfingerLink `json:"links"`
}

func (a *goBlog) apFetchWebfinger(ctx context.Context, user, instance string) (*webfingerResponse, error) {
	wf := &webfingerResponse{}
	pr, pw := io.Pipe()
	go func() {
		err := requests.
			URL(fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s", instance, user, instance)).
			Client(a.httpClient).
			ToWriter(pw).
			Fetch(ctx)
		_ = pw.CloseWithError(err)
	}()
	err := json.NewDecoder(io.LimitReader(pr, 100*bodylimit.KB)).Decode(wf)
	_ = pr.CloseWithError(err)
	if err != nil {
		return nil, fmt.Errorf("failed to query webfinger for %s@%s: %w", user, instance, err)
	}
	return wf, nil
}

func (a *goBlog) apResolveWebfinger(user, instance string) (string, error) {
	wf, err := a.apFetchWebfinger(context.Background(), user, instance)
	if err != nil {
		return "", err
	}
	for _, link := range wf.Links {
		if link.Rel == "self" && (link.Type == "application/activity+json" || link.Type == "application/ld+json") {
			if link.Href != "" {
				return link.Href, nil
			}
		}
	}
	return "", fmt.Errorf("no ActivityPub actor found in webfinger response for %s@%s", user, instance)
}

func (a *goBlog) apAddFollowerManually(blogName, input string) error {
	if _, ok := a.cfg.Blogs[blogName]; !ok {
		return fmt.Errorf("blog not found: %s", blogName)
	}
	actorIRI := input
	// Check if it's a @user@instance handle
	if strings.Contains(input, "@") && !strings.HasPrefix(input, "http") {
		input = strings.TrimPrefix(input, "@")
		parts := strings.SplitN(input, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid handle format: %s (expected @user@instance or user@instance)", input)
		}
		resolved, err := a.apResolveWebfinger(parts[0], parts[1])
		if err != nil {
			return err
		}
		actorIRI = resolved
	}
	// Fetch remote actor
	actor, err := a.apGetRemoteActor(blogName, ap.IRI(actorIRI))
	if err != nil || actor == nil {
		return fmt.Errorf("failed to fetch remote actor %s: %w", actorIRI, err)
	}
	// Get inbox
	inbox := actor.Inbox.GetLink()
	if endpoints := actor.Endpoints; endpoints != nil && endpoints.SharedInbox != nil && endpoints.SharedInbox.GetLink() != "" {
		inbox = endpoints.SharedInbox.GetLink()
	}
	if inbox == "" {
		return fmt.Errorf("actor %s has no inbox", actorIRI)
	}
	if err = a.db.apAddFollower(blogName, actor.GetLink().String(), inbox.String(), apUsername(actor)); err != nil {
		return fmt.Errorf("failed to add follower: %w", err)
	}
	return nil
}

func (a *goBlog) apRefetchFollowers(blogName string) error {
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		return err
	}
	for _, fol := range followers {
		actor, err := a.apGetRemoteActor(blogName, ap.IRI(fol.follower))
		if err != nil || actor == nil {
			a.error("ActivityPub: Failed to retrieve remote actor info", "actor", fol.follower, "err", err)
			continue
		}
		inbox := actor.Inbox.GetLink()
		if endpoints := actor.Endpoints; endpoints != nil && endpoints.SharedInbox != nil && endpoints.SharedInbox.GetLink() != "" {
			inbox = endpoints.SharedInbox.GetLink()
		}
		if inbox == "" {
			a.error("ActivityPub: Failed to get inbox for actor", "actor", fol.follower)
			continue
		}
		username := apUsername(actor)
		if err = a.db.apAddFollower(blogName, actor.GetLink().String(), inbox.String(), username); err != nil {
			a.error("ActivityPub: Failed to update follower info", "err", err)
			return err
		}
	}
	return nil
}

type apFollowerCheckResult struct {
	follower *apFollower
	status   string // "ok", "gone", "moved", "error"
	movedTo  string // if moved, the new account
	err      error
}

func (a *goBlog) apCheckFollowers(blogName string) ([]*apFollowerCheckResult, error) {
	if _, ok := a.cfg.Blogs[blogName]; !ok {
		return nil, fmt.Errorf("blog not found: %s", blogName)
	}
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}
	if len(followers) == 0 {
		return nil, nil
	}
	var results []*apFollowerCheckResult
	for _, fol := range followers {
		result := &apFollowerCheckResult{follower: fol}
		actor, err := a.apGetRemoteActor(blogName, ap.IRI(fol.follower))
		if err != nil || actor == nil {
			result.status = "gone"
			result.err = err
			results = append(results, result)
			continue
		}
		if actor.MovedTo != nil && actor.MovedTo.GetLink() != "" {
			result.status = "moved"
			result.movedTo = actor.MovedTo.GetLink().String()
			results = append(results, result)
			continue
		}
		result.status = "ok"
		results = append(results, result)
	}
	return results, nil
}
