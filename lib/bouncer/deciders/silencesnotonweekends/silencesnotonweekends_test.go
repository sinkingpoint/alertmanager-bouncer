package silencesnotonweekends_test

import (
	"net/http"
	"testing"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silencesnotonweekends"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/testutil"
	"github.com/stretchr/testify/require"
)

func TestSilencesDontExpireOnWeekendsDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         deciders.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Test Expires on Weekend Fails",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silencesnotonweekends.New), nil),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-19T00:23:55.242Z"}`,
			expectedSuccess: false,
		}, {
			name:            "Test Expires on Weekday Works",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silencesnotonweekends.New), nil),
			input:           `{"startsAt":"2020-01-20T00:23:55.242Z", "endsAt": "2020-01-20T00:23:55.242Z"}`,
			expectedSuccess: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := testCase.decider.Decide(testutil.MustMakeRequest(t, http.MethodPost, "/api/v2/silences", testCase.input))

			if testCase.expectedSuccess {
				require.Nil(t, response)
			} else {
				require.NotNil(t, response)
			}
		})
	}
}
