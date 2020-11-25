package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joncrlsn/dque"
)

var apQueue *dque.DQue

type apRequest struct {
	BlogIri, To string
	Activity    []byte
	Try         int
	LastTry     int64
}

func apRequestBuilder() interface{} {
	return &apRequest{}
}

func initAPSendQueue() (err error) {
	queuePath := "queues"
	if _, err := os.Stat(queuePath); os.IsNotExist(err) {
		os.Mkdir(queuePath, 0755)
	}
	apQueue, err = dque.NewOrOpen("activitypub", queuePath, 1000, apRequestBuilder)
	if err != nil {
		return err
	}
	startAPSendQueue()
	return nil
}

func startAPSendQueue() {
	go func() {
		for {
			if rInterface, err := apQueue.PeekBlock(); err == nil {
				if rInterface == nil {
					// Empty request
					_, _ = apQueue.Dequeue()
					continue
				}
				if r, ok := rInterface.(*apRequest); ok {
					if r.LastTry != 0 && time.Now().Before(time.Unix(r.LastTry, 0).Add(time.Duration(r.Try)*10*time.Minute)) {
						apQueue.Enqueue(r)
					} else {
						// Send request
						if err := apSendSigned(r.BlogIri, r.To, r.Activity); err != nil {
							if r.Try++; r.Try < 21 {
								// Try it again
								r.LastTry = time.Now().Unix()
								apQueue.Enqueue(r)
							} else {
								log.Printf("Request to %s failed for the 20th time", r.To)
								log.Println()
								apRemoveInbox(r.To)
							}
						}
					}
					// Finish
					_, _ = apQueue.Dequeue()
					time.Sleep(1 * time.Second)
				} else {
					// Invalid type
					_, _ = apQueue.Dequeue()
				}
			}
		}
	}()
}

func apQueueSendSigned(blogIri, to string, activity interface{}) error {
	body, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	return apQueue.Enqueue(&apRequest{
		BlogIri:  blogIri,
		To:       to,
		Activity: body,
	})
}

func apSendSigned(blogIri, to string, activity []byte) error {
	// Copy activity to sign it
	activityCopy := make([]byte, len(activity))
	copy(activityCopy, activity)
	// Create request context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Create request
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, to, bytes.NewBuffer(activity))
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
	err = apPostSigner.SignRequest(apPrivateKey, blogIri+"#main-key", r, activityCopy)
	apPostSignMutex.Unlock()
	if err != nil {
		return err
	}
	// Do request
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	if !apRequestIsSuccess(resp.StatusCode) {
		body, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return fmt.Errorf("signed request failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
