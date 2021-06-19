package main

import (
	"fmt"
	"io"
	"net/http"
)

func (a *goBlog) healthcheck() bool {
	req, err := http.NewRequest(http.MethodGet, a.getFullAddress("/ping"), nil)
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
