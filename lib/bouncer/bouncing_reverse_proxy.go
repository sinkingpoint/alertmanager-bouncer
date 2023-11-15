package bouncer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/grafana/regexp"
	"github.com/rs/zerolog/log"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"

	"gopkg.in/yaml.v3"
)

type deciderSerialized struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

type bouncerSerialized struct {
	Method   string              `yaml:"method"`
	URIRegex string              `yaml:"uriRegex"`
	Deciders []deciderSerialized `yaml:"deciders"`
	DryRun   bool                `yaml:"dryrun"`
}

// ParseBouncers loads a slice of Bouncers from a given byte array
// which should represent a YAML encoded text stream of serialized bouncers.
func ParseBouncers(b []byte) ([]Bouncer, error) {
	var serializedBouncers struct {
		Bouncers []bouncerSerialized `yaml:"bouncers"`
	}

	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)

	if err := decoder.Decode(&serializedBouncers); err != nil {
		return nil, err
	}

	bouncers := make([]Bouncer, 0, len(serializedBouncers.Bouncers))
	for _, serializedBouncer := range serializedBouncers.Bouncers {
		uriRegex, err := regexp.Compile(serializedBouncer.URIRegex)
		if err != nil {
			return nil, err
		}

		target := Target{
			Method:   serializedBouncer.Method,
			URIRegex: uriRegex,
		}

		deciders := make([]deciders.Decider, 0, len(serializedBouncer.Deciders))
		for _, serializedDecider := range serializedBouncer.Deciders {
			if _, exists := deciderTemplates[serializedDecider.Name]; !exists {
				return nil, fmt.Errorf("no decider template named %q found", serializedDecider.Name)
			}

			decider, err := deciderTemplates[serializedDecider.Name].Make(serializedDecider.Config)
			if err != nil {
				return nil, fmt.Errorf("failed to create decider %q: %s", serializedDecider.Name, err)
			}

			deciders = append(deciders, decider)
		}

		bouncers = append(bouncers, Bouncer{
			Target:   target,
			Deciders: deciders,
			DryRun:   serializedBouncer.DryRun,
		})
	}

	return bouncers, nil
}

// Target Represents a potential target for an HTTP request with both a Method (Which represents the HTTP method), and a URI Regex
// which matches the URI of the request.
type Target struct {
	Method   string
	URIRegex *regexp.Regexp
}

// Matches returns whether the given the given Target matches the given request, i.e. the method matches, and the URI matches the regex.
func (t Target) Matches(req *http.Request) bool {
	methodMatches := strings.EqualFold(req.Method, t.Method)
	uriMatches := t.URIRegex.MatchString(req.URL.RequestURI())
	return methodMatches && uriMatches
}

// Bouncer is a coupling of a Target, and a number of deciders. It can optionally "Bounce" a request, i.e. reject it based on a series of Deciders.
type Bouncer struct {
	Target   Target
	Deciders []deciders.Decider
	DryRun   bool
}

// Bounce takes an HTTPRequest and optionally returns an HTTPError if the request should be "Bounced", i.e. rejected.
func (b Bouncer) Bounce(req *http.Request) *deciders.HTTPError {
	if !b.Target.Matches(req) {
		return nil
	}

	// We want multiple deciders to be able to read the body, so we have to read it here, and then reload it into a buffer for every decider.
	var rawBody []byte
	var err error
	if req.Body == nil || req.Body == http.NoBody {
		rawBody = []byte{}
	} else {
		defer req.Body.Close()
		rawBody, err = io.ReadAll(req.Body)
		if err != nil {
			return &deciders.HTTPError{
				Status: http.StatusBadRequest,
				Err:    "failed to read body from request",
			}
		}
	}

	for _, decider := range b.Deciders {
		req.Body = io.NopCloser(bytes.NewBuffer(rawBody))
		defer req.Body.Close()
		err := decider.Decide(req)
		if err != nil {
			if b.DryRun {
				log.Info().Msgf("Would have rejected %s %s: %s", req.Method, req.URL.RequestURI(), err.Err)
			} else {
				log.Debug().Msgf("Rejected %s %s: %s", req.Method, req.URL.RequestURI(), err.Err)
				return err
			}
		}
	}

	req.Body = io.NopCloser(bytes.NewBuffer(rawBody))
	return nil
}

type bouncingTransport struct {
	backingTransport http.RoundTripper
	bouncers         []Bouncer
}

// SetBouncers updates the bouncers on the given proxy.
func SetBouncers(bouncers []Bouncer, proxy *httputil.ReverseProxy) error {
	transport, ok := proxy.Transport.(bouncingTransport)
	if !ok {
		return fmt.Errorf("given proxy is not a BouncingReverseProxy")
	}

	proxy.Transport = bouncingTransport{
		backingTransport: transport.backingTransport,
		bouncers:         bouncers,
	}

	return nil
}

func (b bouncingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	for _, bouncer := range b.bouncers {
		if err := bouncer.Bounce(request); err != nil {
			return err.ToResponse(), nil
		}
	}
	return b.backingTransport.RoundTrip(request)
}

// NewBouncingReverseProxy generates a ReverseProxy instance which runs the given set of bouncers on every request that passes through it.
func NewBouncingReverseProxy(backend *url.URL, bouncers []Bouncer, backingTransport http.RoundTripper) *httputil.ReverseProxy {
	if backingTransport == nil {
		backingTransport = http.DefaultTransport
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.Transport = bouncingTransport{
		backingTransport: backingTransport,
		bouncers:         bouncers,
	}

	return proxy
}
