package silenceshaveauthor_test

import (
	"net/http"
	"testing"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silenceshaveauthor"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/testutil"
	"github.com/stretchr/testify/require"
)

func TestAllSilencesHaveAuthorDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         deciders.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Bad Domain Fails",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveauthor.New), map[string]interface{}{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test","createdBy":"colin@quirl.co.nz", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: false,
		},
		{
			name:            "No Domain Doesn't Die",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveauthor.New), map[string]interface{}{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: false,
		},
		{
			name:            "Correct Domain Passes",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveauthor.New), map[string]interface{}{"domain": "@cloudflare.com"}),
			input:           `{"comment":"test", "createdBy": "colin@cloudflare.com", "startsAt":"2020-01-21T00:23:55.242Z", "endsAt":"2020-01-21T01:23:55.242Z"}`,
			expectedSuccess: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			response := tt.decider.Decide(testutil.MustMakeRequest(t, http.MethodPost, "/api/v2/silences", tt.input))

			if tt.expectedSuccess {
				require.Nil(t, response)
			} else {
				require.NotNil(t, response)
			}
		})
	}
}
