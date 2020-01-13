package bouncer

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type bouncingTransport struct {
	backingTransport http.RoundTripper
}

func (b bouncingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return b.backingTransport.RoundTrip(request)
}

func NewBouncingReverseProxy(backend *url.URL, backingTransport http.RoundTripper) *httputil.ReverseProxy {
	if backingTransport == nil {
		backingTransport = http.DefaultTransport
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.Transport = bouncingTransport{
		backingTransport: backingTransport,
	}

	return proxy
}
