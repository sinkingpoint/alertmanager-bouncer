package deciders

import (
	"io"
	"net/http"
	"strings"
)

// Decider is a function which takes an HTTP request and optionally returns an HTTPError, if the given request should be rejected.
type Decider interface {
	Decide(req *http.Request) *HTTPError
}

type DeciderFunc func(req *http.Request) *HTTPError

func (d DeciderFunc) Decide(req *http.Request) *HTTPError {
	return d(req)
}

// HTTPError represents an error, coupled with an HTTP Status Code.
type HTTPError struct {
	Status int
	Err    string
}

// ToResponse converts the given HTTPError into an HTTP Response, which can be sent back to a client.
func (h *HTTPError) ToResponse() *http.Response {
	return &http.Response{
		StatusCode: h.Status,
		Body:       io.NopCloser(strings.NewReader(h.Err)),
	}
}

type Template interface {
	Make(map[string]interface{}) (Decider, error)
}

type TemplateFunc func(map[string]interface{}) (Decider, error)

func (d TemplateFunc) Make(config map[string]interface{}) (Decider, error) {
	return d(config)
}
