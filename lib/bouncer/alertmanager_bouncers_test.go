package bouncer_test

import "testing"

import "github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"

import "net/http"

import "io/ioutil"

import "bytes"

func mustBuildRequest(input string, t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", "localhost", ioutil.NopCloser(bytes.NewBufferString(input)))
	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	defer req.Body.Close()
	return req
}

func TestAllSilencesHaveAuthorDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         bouncer.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Bad Domain Fails",
			decider:         bouncer.AllSilencesHaveAuthorDecider(map[string]string{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test","createdBy":"colin@quirl.co.nz", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: false,
		},
		{
			name:            "No Domain Doesn't Die",
			decider:         bouncer.AllSilencesHaveAuthorDecider(map[string]string{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: false,
		},
		{
			name:            "Correct Domain Passes",
			decider:         bouncer.AllSilencesHaveAuthorDecider(map[string]string{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test", "createdBy": "colin@cloudflare.com", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: true,
		},
	}

	for _, testCase := range testCases {
		response := testCase.decider(mustBuildRequest(testCase.input, t))
		if (response == nil) != testCase.expectedSuccess {
			var errorText string
			if response != nil {
				errorText = response.Err.Error()
			} else {
				errorText = ""
			}
			t.Errorf("Test %s failed. Expected %t got %t. Debug: %s", testCase.name, testCase.expectedSuccess, !testCase.expectedSuccess, errorText)
		}
	}
}
