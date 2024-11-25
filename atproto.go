package main

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) initAtproto() {
	a.pPostHooks = append(a.pPostHooks, a.atprotoPost)
	a.pDeleteHooks = append(a.pDeleteHooks, a.atprotoDelete)
	a.pUndeleteHooks = append(a.pUndeleteHooks, a.atprotoPost)
}

func (at *configAtproto) enabled() bool {
	if at == nil || !at.Enabled || at.Handle == "" || at.Password == "" {
		return false
	}
	return true
}

const (
	atprotoUriParam   = "atprotouri"
	atprotoUriPattern = `^at://([^/]+)/([^/]+)/([^/]+)$`
)

func (a *goBlog) atprotoPost(p *post) {
	if atproto := a.getBlogFromPost(p).Atproto; atproto.enabled() && p.isPublicPublishedSectionPost() {
		session, err := a.createAtprotoSession(atproto)
		if err != nil {
			a.error("Failed to create ATProto session", "err", err)
			return
		}
		atp := a.toAtprotoPost(atproto, p)
		resp, err := a.publishPost(atproto, session, atp)
		if err != nil {
			a.error("Failed to send post to ATProto", "err", err)
			return
		}
		if resp.URI == "" {
			// Not published
			return
		}
		// Save URI to post
		if err := a.db.replacePostParam(p.Path, atprotoUriParam, []string{resp.URI}); err != nil {
			a.error("Failed to save ATProto URI", "err", err)
		}
		return
	}
}

func (a *goBlog) atprotoDelete(p *post) {
	if atproto := a.getBlogFromPost(p).Atproto; atproto.enabled() {
		atprotouri := p.firstParameter(atprotoUriParam)
		if atprotouri == "" {
			return
		}
		// Delete record
		session, err := a.createAtprotoSession(atproto)
		if err != nil {
			a.error("Failed to create ATProto session", "err", err)
			return
		}
		if err := a.deleteAtprotoRecord(atproto, session, atprotouri); err != nil {
			a.error("Failed to delete ATProto record", "err", err)
		}
		// Delete URI from post
		if err := a.db.replacePostParam(p.Path, atprotoUriParam, []string{}); err != nil {
			a.error("Failed to remove ATProto URI", "err", err)
		}
		return
	}
}

func (at *configAtproto) pdsURL() string {
	return cmp.Or(at.Pds, "https://bsky.social")
}

type atprotoSessionResponse struct {
	AccessToken string `json:"accessJwt"` // JWT access token.
	UserID      string `json:"did"`       // User identifier.
}

func (a *goBlog) createAtprotoSession(atproto *configAtproto) (*atprotoSessionResponse, error) {
	var response atprotoSessionResponse
	err := requests.URL(atproto.pdsURL() + "/xrpc/com.atproto.server.createSession").
		Method(http.MethodPost).
		Client(a.httpClient).
		BodyJSON(map[string]string{
			"identifier": atproto.Handle,
			"password":   atproto.Password,
		}).
		ContentType(contenttype.JSON).
		ToJSON(&response).
		Fetch(context.Background())
	if err != nil {
		return nil, err
	}
	return &response, nil
}

type atprotoPublishResponse struct {
	URI string `json:"uri"`
}

func (a *goBlog) publishPost(atproto *configAtproto, session *atprotoSessionResponse, atpost *atprotoPost) (*atprotoPublishResponse, error) {
	var resp atprotoPublishResponse
	err := requests.URL(atproto.pdsURL()+"/xrpc/com.atproto.repo.createRecord").
		Method(http.MethodPost).
		Client(a.httpClient).
		Header("Authorization", "Bearer "+session.AccessToken).
		BodyJSON(map[string]any{
			"repo":       session.UserID,
			"collection": "app.bsky.feed.post",
			"record":     atpost,
		}).
		ContentType(contenttype.JSON).
		ToJSON(&resp).
		Fetch(context.Background())
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *goBlog) deleteAtprotoRecord(atproto *configAtproto, session *atprotoSessionResponse, uri string) error {
	re := regexp.MustCompile(atprotoUriPattern)
	matches := re.FindStringSubmatch(uri)
	if matches == nil || len(matches) != 4 {
		return fmt.Errorf("invalid URI format")
	}
	return requests.URL(atproto.pdsURL()+"/xrpc/com.atproto.repo.deleteRecord").
		Method(http.MethodPost).
		Client(a.httpClient).
		Header("Authorization", "Bearer "+session.AccessToken).
		BodyJSON(map[string]any{
			"repo":       matches[1],
			"collection": matches[2],
			"rkey":       matches[3],
		}).
		ContentType(contenttype.JSON).
		Fetch(context.Background())
}

type atprotoPost struct {
	Type      string          `json:"$type"`
	Text      string          `json:"text"`
	CreatedAt string          `json:"createdAt"`
	Langs     []string        `json:"langs,omitempty"`
	Embed     *atprotoEmbed   `json:"embed,omitempty"`
	Facets    []*atprotoFacet `json:"facets,omitempty"`
}

type atprotoEmbed struct {
	Type     string                `json:"$type"`
	External *atprotoEmbedExternal `json:"external,omitempty"`
}

type atprotoEmbedExternal struct {
	URI         string `json:"uri"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type atprotoFacet struct {
	Features []atprotoFeature `json:"features"`
	Index    atprotoIndex     `json:"index"`
}

type atprotoFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri,omitempty"`
	Tag  string `json:"tag,omitempty"`
}

type atprotoIndex struct {
	ByteEnd   int `json:"byteEnd"`
	ByteStart int `json:"byteStart"`
}

func (a *goBlog) toAtprotoPost(atp *configAtproto, p *post) *atprotoPost {
	postTitle := cmp.Or(p.RenderedTitle, a.fallbackTitle(p))
	postDescription := a.postSummary(p)
	bc := a.getBlogFromPost(p)
	result := &atprotoPost{
		Type:      "app.bsky.feed.post",
		CreatedAt: cmp.Or(toLocalSafe(p.Published), time.Now().Format(time.RFC3339)),
		Langs:     []string{bc.Lang},
		Embed: &atprotoEmbed{
			Type: "app.bsky.embed.external",
			External: &atprotoEmbedExternal{
				URI:         a.getFullAddress(p.Path),
				Title:       cmp.Or(postTitle, "-"),
				Description: cmp.Or(postDescription, "-"),
			},
		},
	}
	// Build text of ATProto post
	builder := builderpool.Get()
	defer builderpool.Put(builder)
	facets := []*atprotoFacet{}
	// Add title first and add two line breaks
	if postTitle != "" {
		builder.WriteString(postTitle)
		builder.WriteString("\n\n")
	}
	// Add short link
	start := builder.Len()
	link := a.shortPostURL(p)
	builder.WriteString(link)
	end := builder.Len()
	facets = append(facets, &atprotoFacet{
		Features: []atprotoFeature{{
			Type: "app.bsky.richtext.facet#link",
			URI:  link,
		}},
		Index: atprotoIndex{
			ByteStart: start,
			ByteEnd:   end,
		},
	})
	// Add hashtags
	if len(atp.TagsTaxonomies) == 0 {
		atp.TagsTaxonomies = append(atp.TagsTaxonomies, "tags")
	}
	firstTag := true
	for _, tagTax := range atp.TagsTaxonomies {
		for _, tag := range p.Parameters[tagTax] {
			if firstTag {
				_, _ = builder.WriteString("\n\n")
				firstTag = false
			} else {
				_, _ = builder.WriteString(" ")
			}
			start = builder.Len()
			builder.WriteString("#" + tag)
			end = builder.Len()
			facets = append(facets, &atprotoFacet{
				Features: []atprotoFeature{{
					Type: "app.bsky.richtext.facet#tag",
					Tag:  tag,
				}},
				Index: atprotoIndex{
					ByteStart: start,
					ByteEnd:   end,
				},
			})
		}
	}
	// Set result text
	result.Text = builder.String()
	result.Facets = facets
	return result
}
