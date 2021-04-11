package main

import (
	"fmt"
	"io"
	"net/http"
)

func healthcheck() bool {
	req, err := http.NewRequest(http.MethodGet, appConfig.Server.PublicAddress+"/ping", nil)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	resp, err := appHttpClient.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200
}
