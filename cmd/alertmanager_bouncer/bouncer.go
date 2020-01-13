package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
)

type config struct {
	listenAddr  *url.URL
	backendAddr *url.URL
}

func main() {
	backendURL, _ := url.Parse("http://quirl.co.nz")
	cats := bouncer.NewBouncingReverseProxy(backendURL, nil)
	server := http.Server{
		Handler: cats,
		Addr:    "localhost:8080",
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}
