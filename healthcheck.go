package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (a *goBlog) healthcheck() bool {
	timeoutContext, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()
	req, err := http.NewRequestWithContext(timeoutContext, http.MethodGet, a.getFullAddress("/ping"), nil)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200
}

func (a *goBlog) healthcheckExitCode() int {
	if a.healthcheck() {
		return 0
	} else {
		return 1
	}
}
