package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

type apRequest struct {
	BlogIri, To string
	Activity    []byte
	Try         int
}

func (a *goBlog) initAPSendQueue() {
	a.listenOnQueue("ap", 30*time.Second, func(qi *queueItem, dequeue func(), reschedule func(time.Duration)) {
		var r apRequest
		if err := gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&r); err != nil {
			log.Println("activitypub queue:", err.Error())
			dequeue()
			return
		}
		if err := a.apSendSigned(r.BlogIri, r.To, r.Activity); err != nil {
			if r.Try++; r.Try < 20 {
				// Try it again
				buf := bufferpool.Get()
				_ = r.encode(buf)
				qi.content = buf.Bytes()
				reschedule(time.Duration(r.Try) * 10 * time.Minute)
				bufferpool.Put(buf)
				return
			}
			log.Println("AP request failed for the 20th time:", r.To)
			_ = a.db.apRemoveInbox(r.To)
		}
		dequeue()
	})
}

func (a *goBlog) apQueueSendSigned(blogIri, to string, activity any) error {
	body, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(activity)
	if err != nil {
		return err
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := (&apRequest{
		BlogIri:  blogIri,
		To:       to,
		Activity: body,
	}).encode(buf); err != nil {
		return err
	}
	return a.enqueue("ap", buf.Bytes(), time.Now())
}

func (r *apRequest) encode(w io.Writer) error {
	return gob.NewEncoder(w).Encode(r)
}

func (a *goBlog) apSendSigned(blogIri, to string, activity []byte) error {
	// Create request context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Create request
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, to, bytes.NewReader(activity))
	if err != nil {
		return err
	}
	r.Header.Set("Accept-Charset", "utf-8")
	r.Header.Set("Accept", contenttype.ASUTF8)
	r.Header.Set(contentType, contenttype.ASUTF8)
	// Sign request
	if err = a.signRequest(r, blogIri); err != nil {
		return err
	}
	// Do request
	resp, err := a.httpClient.Do(r)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if !apRequestIsSuccess(resp.StatusCode) {
		return fmt.Errorf("signed request failed with status %d", resp.StatusCode)
	}
	return nil
}
