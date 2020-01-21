package bouncer_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
)

func mustMakeRequest(t *testing.T, method string, urlString string, body string) *http.Request {
	url, err := url.Parse(urlString)
	if err != nil {
		t.Fatalf(err.Error())
	}

	return &http.Request{
		Method: method,
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestParseBouncers(t *testing.T) {
	testCases := []struct {
		serialized          string
		expectedNumBouncers int
		expectedNumDeciders []int
		expectedError       bool
	}{
		{
			serialized:          `{"bouncers": [{"method": "POST", "uriRegex":"cats", deciders: []}]}`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{0},
			expectedError:       false,
		},
		{
			serialized:          `{"bouncers": [{"method": "POST", "uriRegex":"cats", deciders: [{"name": "AllSilencesHaveAuthor", "config":{}}]}]}`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{1},
			expectedError:       true,
		},
		{
			serialized:          `{"bouncers": [{"method": "POST", "uriRegex":"cats", deciders: [{"name": "AllSilencesHaveAuthor", "config":{"domain":"quirl.co.nz"}}]}]}`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{1},
			expectedError:       false,
		},
		{
			serialized:          `{"bouncers": [{"method": "POST", "uriRegex":"cats", deciders: [{"name": "AllSilencesHaveAutho", "config":{"domain":"quirl.co.nz"}}]}]}`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{1},
			expectedError:       true,
		},
		{
			serialized:          `{"bouncers": [{"method": "POST", "uriRegex":"cats", deciders: [{"name": "AllSilencesHaveAuthor", "config":{}}]}]}`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{1},
			expectedError:       true,
		},
	}

	for _, testCase := range testCases {
		bouncers, err := bouncer.ParseBouncers([]byte(testCase.serialized))
		if err != nil {
			if !testCase.expectedError {
				t.Errorf("Got an error parsing bouncers, but didn't expect one: %s", err.Error())
			}
			continue
		} else if err == nil && testCase.expectedError {
			t.Errorf("Expected an error, but didn't get one")
			continue
		}

		if len(bouncers) != testCase.expectedNumBouncers {
			t.Errorf("Expected to load %d bouncers, but loaded %d", testCase.expectedNumBouncers, len(bouncers))
			continue
		}

		for i, bouncer := range bouncers {
			if len(bouncer.Deciders) != testCase.expectedNumDeciders[i] {
				t.Errorf("Expected bouncer %d to load %d deciders, but loaded %d", i, testCase.expectedNumDeciders, len(bouncer.Deciders))
			}
		}
	}
}

func TestTargetMatches(t *testing.T) {
	testCases := []struct {
		name           string
		target         bouncer.Target
		request        *http.Request
		expectedOutput bool
	}{
		{
			name: "Test method normalization",
			target: bouncer.Target{
				Method:   "poST",
				URIRegex: regexp.MustCompile("/api/v1/silences"),
			},
			request:        mustMakeRequest(t, "POsT", "http://testendpoint/api/v1/silences", ""),
			expectedOutput: true,
		},
		{
			name: "Test regexp is applied",
			target: bouncer.Target{
				Method:   "POST",
				URIRegex: regexp.MustCompile("^/api/v[12]/silences$"),
			},
			request:        mustMakeRequest(t, "POST", "http://testendpoint/api/v1/silences", ""),
			expectedOutput: true,
		},
		{
			name: "Test doesn't match invalid",
			target: bouncer.Target{
				Method:   "POST",
				URIRegex: regexp.MustCompile("/api/v/silences"),
			},
			request:        mustMakeRequest(t, "POST", "http://testendpoint/api/v1/silences", ""),
			expectedOutput: false,
		},
	}

	for _, testCase := range testCases {
		actualOutput := testCase.target.Matches(testCase.request)
		if actualOutput != testCase.expectedOutput {
			t.Errorf("Test '%s' failed - expected %t got %t\n", testCase.name, testCase.expectedOutput, actualOutput)
		}
	}
}

func TestBouncerBounces(t *testing.T) {
	// Heavily cribbed from https://golang.org/src/net/http/httputil/reverseproxy_test.go

	// Setup a backend server to proxy requests to
	const backendResponse = "I am the backend"
	const backendStatus = 404
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(backendStatus)
		w.Write([]byte(backendResponse))
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	testCases := [...]struct {
		name           string
		bouncers       []bouncer.Bouncer
		requestMethod  string
		requestURI     string
		requestBody    string
		expectedStatus int
		expectedOutput string
	}{
		{
			name:           "Test No Bouncers Proxies",
			bouncers:       []bouncer.Bouncer{},
			requestMethod:  "GET",
			requestURI:     "/api/v1/silences",
			requestBody:    "",
			expectedStatus: backendStatus,
			expectedOutput: backendResponse,
		},
		{
			name: "Test Reject All Bouncer Rejects",
			bouncers: []bouncer.Bouncer{
				bouncer.Bouncer{
					Target: bouncer.Target{
						Method:   "GET",
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []bouncer.Decider{
						func(req *http.Request) *bouncer.HTTPError {
							return &bouncer.HTTPError{
								Err:    fmt.Errorf("No"),
								Status: 401,
							}
						},
					},
				},
			},
			requestMethod:  "GET",
			requestBody:    "",
			expectedStatus: 401,
			expectedOutput: "No",
		},
		{
			name: "Test Accept All Bouncer Accepts",
			bouncers: []bouncer.Bouncer{
				bouncer.Bouncer{
					Target: bouncer.Target{
						Method:   "GET",
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []bouncer.Decider{
						func(req *http.Request) *bouncer.HTTPError {
							return nil
						},
					},
				},
			},
			requestMethod:  "GET",
			requestBody:    "",
			expectedStatus: backendStatus,
			expectedOutput: backendResponse,
		},
		{
			// Considering the Read-Once nature of Body objects (Readers)
			// This test makes sure that if one Decider reads the body, a subsequent one can as well
			name: "Test Multiple Bouncers Can Read Body",
			bouncers: []bouncer.Bouncer{
				bouncer.Bouncer{
					Target: bouncer.Target{
						Method:   "GET",
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []bouncer.Decider{
						func(req *http.Request) *bouncer.HTTPError {
							body, err := ioutil.ReadAll(req.Body)
							if err != nil {
								return &bouncer.HTTPError{
									Err:    err,
									Status: 400,
								}
							}

							if len(body) == 0 {
								fmt.Printf("Body is 0 in first\n")
								return &bouncer.HTTPError{
									Err:    fmt.Errorf("No"),
									Status: 400,
								}
							}

							return nil
						},
						func(req *http.Request) *bouncer.HTTPError {
							body, err := ioutil.ReadAll(req.Body)
							if err != nil {
								return &bouncer.HTTPError{
									Err:    err,
									Status: 400,
								}
							}

							if len(body) == 0 {
								fmt.Printf("Body is 0 in second\n")
								return &bouncer.HTTPError{
									Err:    fmt.Errorf("No"),
									Status: 400,
								}
							}

							return nil
						},
					},
				},
			},
			requestMethod:  "GET",
			requestBody:    "TestBody",
			expectedStatus: backendStatus,
			expectedOutput: backendResponse,
		},
	}

	for _, testCase := range testCases {
		bouncer := bouncer.NewBouncingReverseProxy(backendURL, testCase.bouncers, http.DefaultTransport)
		frontend := httptest.NewServer(bouncer)
		defer frontend.Close()
		frontendClient := frontend.Client()

		request := mustMakeRequest(t, testCase.requestMethod, frontend.URL+testCase.requestURI, testCase.requestBody)
		response, err := frontendClient.Do(request)
		if err != nil {
			t.Error(err)
		}

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			t.Error(err)
		}

		if string(body) != testCase.expectedOutput || response.StatusCode != testCase.expectedStatus {
			t.Errorf("Test '%s' failed - expected (%d, %s) but got (%d, %s)\n", testCase.name, testCase.expectedStatus, testCase.expectedOutput, response.StatusCode, string(body))
		}
	}
}
