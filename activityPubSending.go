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
	"time"
)

type apRequest struct {
	BlogIri, To string
	Activity    []byte
	Try         int
}

func initAPSendQueue() {
	go func() {
		for {
			qi, err := peekQueue("ap")
			if err != nil {
				log.Println(err.Error())
				continue
			} else if qi != nil {
				var r apRequest
				err = gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&r)
				if err != nil {
					log.Println(err.Error())
					_ = qi.dequeue()
					continue
				}
				if err := apSendSigned(r.BlogIri, r.To, r.Activity); err != nil {
					if r.Try++; r.Try < 20 {
						// Try it again
						qi.content, _ = r.encode()
						_ = qi.reschedule(time.Duration(r.Try) * 10 * time.Minute)
						continue
					} else {
						log.Printf("Request to %s failed for the 20th time", r.To)
						log.Println()
						_ = apRemoveInbox(r.To)
					}
				}
				err = qi.dequeue()
				if err != nil {
					log.Println(err.Error())
				}
			} else {
				// No item in the queue, wait a moment
				time.Sleep(15 * time.Second)
			}
		}
	}()
}

func apQueueSendSigned(blogIri, to string, activity interface{}) error {
	body, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	b, err := (&apRequest{
		BlogIri:  blogIri,
		To:       to,
		Activity: body,
	}).encode()
	if err != nil {
		return err
	}
	return enqueue("ap", b, time.Now())
}

func (r *apRequest) encode() ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(r)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func apSendSigned(blogIri, to string, activity []byte) error {
	// Create request context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Create request
	var requestBuffer bytes.Buffer
	requestBuffer.Write(activity)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, to, &requestBuffer)
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
	r.Header.Set("Accept", contentTypeASUTF8)
	r.Header.Set(contentType, contentTypeASUTF8)
	r.Header.Set("Host", iri.Host)
	// Sign request
	apPostSignMutex.Lock()
	err = apPostSigner.SignRequest(apPrivateKey, blogIri+"#main-key", r, activity)
	apPostSignMutex.Unlock()
	if err != nil {
		return err
	}
	// Do request
	resp, err := appHttpClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !apRequestIsSuccess(resp.StatusCode) {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("signed request failed with status %d: %s", resp.StatusCode, string(body))
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return nil
}
