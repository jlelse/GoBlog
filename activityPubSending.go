package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

type apRequest struct {
	BlogIri, To string
	Activity    []byte
	Try         int
}

func (a *goBlog) initAPSendQueue() {
	go func() {
		done := false
		var wg sync.WaitGroup
		wg.Add(1)
		a.shutdown.Add(func() {
			done = true
			wg.Wait()
			log.Println("Stopped AP send queue")
		})
		for !done {
			qi, err := a.db.peekQueue("ap")
			if err != nil {
				log.Println("activitypub send queue:", err.Error())
				continue
			}
			if qi == nil {
				// No item in the queue, wait a moment
				time.Sleep(5 * time.Second)
				continue
			}
			var r apRequest
			if err = gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&r); err != nil {
				log.Println("activitypub send queue:", err.Error())
				_ = a.db.dequeue(qi)
				continue
			}
			if err = a.apSendSigned(r.BlogIri, r.To, r.Activity); err != nil {
				if r.Try++; r.Try < 20 {
					// Try it again
					buf := bufferpool.Get()
					_ = r.encode(buf)
					qi.content = buf.Bytes()
					_ = a.db.reschedule(qi, time.Duration(r.Try)*10*time.Minute)
					bufferpool.Put(buf)
					continue
				}
				log.Println("AP request failed for the 20th time:", r.To)
				_ = a.db.apRemoveInbox(r.To)
			}
			if err = a.db.dequeue(qi); err != nil {
				log.Println("activitypub send queue:", err.Error())
			}
		}
		wg.Done()
	}()
}

func (db *database) apQueueSendSigned(blogIri, to string, activity interface{}) error {
	body, err := json.Marshal(activity)
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
	return db.enqueue("ap", buf.Bytes(), time.Now())
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
	iri, err := url.Parse(to)
	if err != nil {
		return err
	}
	r.Header.Set("Accept-Charset", "utf-8")
	r.Header.Set("Date", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05")+" GMT")
	r.Header.Set(userAgent, appUserAgent)
	r.Header.Set("Accept", contenttype.ASUTF8)
	r.Header.Set(contentType, contenttype.ASUTF8)
	r.Header.Set("Host", iri.Host)
	// Sign request
	a.apPostSignMutex.Lock()
	err = a.apPostSigner.SignRequest(a.apPrivateKey, blogIri+"#main-key", r, activity)
	a.apPostSignMutex.Unlock()
	if err != nil {
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
