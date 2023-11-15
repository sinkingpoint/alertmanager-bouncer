package testutil

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
)

func MustMakeRequest(t *testing.T, method, urlString, body string) *http.Request {
	t.Helper()
	url, err := url.Parse(urlString)
	if err != nil {
		t.Fatalf(err.Error())
	}

	return &http.Request{
		Method: method,
		URL:    url,
		Body:   io.NopCloser(bytes.NewBufferString(body)),
	}
}

func MustMakeDecider(t *testing.T, template deciders.Template, config map[string]interface{}) deciders.Decider {
	t.Helper()
	decider, err := template.Make(config)
	if err != nil {
		t.Fatalf(err.Error())
	}

	return decider
}
