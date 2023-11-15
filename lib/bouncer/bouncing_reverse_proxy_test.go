package bouncer_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/grafana/regexp"
	"github.com/stretchr/testify/require"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/testutil"
)

func TestParseBouncers(t *testing.T) {
	testCases := []struct {
		serialized          string
		expectedNumBouncers int
		expectedNumDeciders []int
		expectedError       bool
	}{
		{
			serialized: `
bouncers:
 - method: "POST"
   uriRegex: "cats"
   deciders: []
`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{0},
			expectedError:       false,
		},
		{
			serialized: `
bouncers:
  - method: "POST"
	uriRegex: "cats"
	deciders:
	  - name: AllSilencesHaveAuthor
		config: {}`,
			expectedNumBouncers: 0,
			expectedNumDeciders: []int{0},
			expectedError:       true,
		},
		{
			serialized: `
bouncers:
  - method: "POST"
    uriRegex: "cats"
    deciders:
      - name: AllSilencesHaveAuthor
        config:
          domain: "quirl.co.nz"
`,
			expectedNumBouncers: 1,
			expectedNumDeciders: []int{1},
			expectedError:       false,
		},
		{
			serialized: `
bouncers:
  - method: "POST"
    uriRegex: "cats"
    deciders:
      - name: AllSilencesHaveAuthor
        config:
          - domain: "quirl.co.nz"
`,
			expectedNumBouncers: 0,
			expectedNumDeciders: []int{0},
			expectedError:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("Test %s", testCase.serialized), func(t *testing.T) {
			bouncers, err := bouncer.ParseBouncers([]byte(testCase.serialized))

			if testCase.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, bouncers, testCase.expectedNumBouncers, "Unexpected number of bouncers")

			for i, bouncer := range bouncers {
				require.Len(t, bouncer.Deciders, testCase.expectedNumDeciders[i], "Unexpected number of deciders in bouncer %d", i)
			}
		})

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
			request:        testutil.MustMakeRequest(t, http.MethodPost, "http://testendpoint/api/v1/silences", ""),
			expectedOutput: true,
		},
		{
			name: "Test regexp is applied",
			target: bouncer.Target{
				Method:   http.MethodPost,
				URIRegex: regexp.MustCompile("^/api/v[12]/silences$"),
			},
			request:        testutil.MustMakeRequest(t, http.MethodPost, "http://testendpoint/api/v1/silences", ""),
			expectedOutput: true,
		},
		{
			name: "Test doesn't match invalid",
			target: bouncer.Target{
				Method:   http.MethodPost,
				URIRegex: regexp.MustCompile("/api/v/silences"),
			},
			request:        testutil.MustMakeRequest(t, http.MethodPost, "http://testendpoint/api/v1/silences", ""),
			expectedOutput: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expectedOutput, testCase.target.Matches(testCase.request))
		})
	}
}

func TestBouncerBounces(t *testing.T) {
	// Heavily cribbed from https://golang.org/src/net/http/httputil/reverseproxy_test.go

	// Setup a backend server to proxy requests to
	const backendResponse = "I am the backend"
	const backendStatus = http.StatusNotFound
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(backendStatus)
		w.Write([]byte(backendResponse))
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	require.NoError(t, err)

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
			requestMethod:  http.MethodGet,
			requestURI:     "/api/v1/silences",
			requestBody:    "",
			expectedStatus: backendStatus,
			expectedOutput: backendResponse,
		},
		{
			name: "Test Reject All Bouncer Rejects",
			bouncers: []bouncer.Bouncer{
				{
					Target: bouncer.Target{
						Method:   http.MethodGet,
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []deciders.Decider{
						deciders.DeciderFunc(func(req *http.Request) *deciders.HTTPError {
							return &deciders.HTTPError{
								Err:    "No",
								Status: http.StatusBadRequest,
							}
						}),
					},
				},
			},
			requestMethod:  http.MethodGet,
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
			expectedOutput: "No",
		},
		{
			name: "Test Accept All Bouncer Accepts",
			bouncers: []bouncer.Bouncer{
				{
					Target: bouncer.Target{
						Method:   http.MethodGet,
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []deciders.Decider{
						deciders.DeciderFunc(func(req *http.Request) *deciders.HTTPError {
							return nil
						}),
					},
				},
			},
			requestMethod:  http.MethodGet,
			requestBody:    "",
			expectedStatus: backendStatus,
			expectedOutput: backendResponse,
		},
		{
			// Considering the Read-Once nature of Body objects (Readers)
			// This test makes sure that if one Decider reads the body, a subsequent one can as well
			name: "Test Multiple Bouncers Can Read Body",
			bouncers: []bouncer.Bouncer{
				{
					Target: bouncer.Target{
						Method:   http.MethodGet,
						URIRegex: regexp.MustCompile(".*"),
					},
					Deciders: []deciders.Decider{
						deciders.DeciderFunc(func(req *http.Request) *deciders.HTTPError {
							body, err := io.ReadAll(req.Body)
							if err != nil {
								return &deciders.HTTPError{
									Err:    err.Error(),
									Status: http.StatusBadRequest,
								}
							}

							if len(body) == 0 {
								fmt.Printf("Body is 0 in first\n")
								return &deciders.HTTPError{
									Err:    "No",
									Status: http.StatusBadRequest,
								}
							}

							return nil
						}),
						deciders.DeciderFunc(func(req *http.Request) *deciders.HTTPError {
							body, err := io.ReadAll(req.Body)
							if err != nil {
								return &deciders.HTTPError{
									Err:    err.Error(),
									Status: 400,
								}
							}

							if len(body) == 0 {
								fmt.Printf("Body is 0 in second\n")
								return &deciders.HTTPError{
									Err:    "No",
									Status: 400,
								}
							}

							return nil
						}),
					},
				},
			},
			requestMethod:  http.MethodGet,
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

		request := testutil.MustMakeRequest(t, testCase.requestMethod, frontend.URL+testCase.requestURI, testCase.requestBody)
		response, err := frontendClient.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)

		require.Equal(t, testCase.expectedOutput, string(body))
		require.Equal(t, testCase.expectedStatus, response.StatusCode)
	}
}
