package bouncer_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer"
)

func mustBuildRequest(input string, t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", "localhost", ioutil.NopCloser(bytes.NewBufferString(input)))
	req.Header.Add("X-Test", "Cats")
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

func TestMirrorDecider(t *testing.T) {
	// Setup a backend server to proxy requests to
	recChan := make(chan int, 1)
	const backendResponse = "I am the backend"
	const backendStatus = 404
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(backendStatus)
		w.Write([]byte(backendResponse))
		if r.Header.Get("X-Test") == "Cats" {
			recChan <- 1
		}
	}))
	defer backend.Close()

	decider := bouncer.MirrorDecider(map[string]string{"destination": backend.URL})
	err := decider(mustBuildRequest("", t))
	if err != nil {
		t.Fatalf("Got error mirroring reqest: %s", err.Err.Error())
	}
	// Here we assume that 500ms is enough for the request to flow through the kernel networking stack
	// This might be flaky on slow hardware I guess
	time.Sleep(time.Duration(500) * time.Millisecond)
	if len(recChan) == 0 {
		t.Errorf("Expected request to be mirrored to the backend, but it wasn't")
	}
}

func TestSilencesDontExpireOnWeekendsDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         bouncer.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Test Expires on Weekend Fails",
			decider:         bouncer.SilencesDontExpireOnWeekendsDecider(nil),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-19T00:23:55.242Z"}`,
			expectedSuccess: false,
		}, {
			name:            "Test Expires on Weekday Works",
			decider:         bouncer.SilencesDontExpireOnWeekendsDecider(nil),
			input:           `{"startsAt":"2020-01-20T00:23:55.242Z", "endsAt": "2020-01-20T00:23:55.242Z"}`,
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

func TestLongSilencesHaveTicketDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         bouncer.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Test Short Silences Work",
			decider:         bouncer.LongSilencesHaveTicketDecider(map[string]string{"maxLength": "8h"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-19T00:23:55.242Z"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences Without ticket get rejected",
			decider:         bouncer.LongSilencesHaveTicketDecider(map[string]string{"maxLength": "8h"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z",}`,
			expectedSuccess: false,
		},
		{
			name:            "Test Long Silences With Ticket Work",
			decider:         bouncer.LongSilencesHaveTicketDecider(map[string]string{"maxLength": "8h"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "TICKET-1"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences With Custom Ticket Work",
			decider:         bouncer.LongSilencesHaveTicketDecider(map[string]string{"maxLength": "8h", "ticketRegex": "CATS"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "CATS"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences With Custom Ticket Work",
			decider:         bouncer.LongSilencesHaveTicketDecider(map[string]string{"maxLength": "8h", "ticketRegex": "CATS1"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "CATS"}`,
			expectedSuccess: false,
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
