package silenceshaveticket_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/silenceshaveticket"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/testutil"
	"github.com/stretchr/testify/require"
)

func TestLongSilencesHaveTicketDecider(t *testing.T) {
	testCases := []struct {
		name            string
		decider         deciders.Decider
		input           string
		expectedSuccess bool
	}{
		{
			name:            "Test Short Silences Work",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveticket.New), map[string]interface{}{"maxLength": 8 * time.Hour}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-19T00:23:55.242Z"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences Without ticket get rejected",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveticket.New), map[string]interface{}{"maxLength": 8 * time.Hour}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z",}`,
			expectedSuccess: false,
		},
		{
			name:            "Test Long Silences With Ticket Work",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveticket.New), map[string]interface{}{"maxLength": 8 * time.Hour}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "TICKET-1"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences With Custom Ticket Work",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveticket.New), map[string]interface{}{"maxLength": 8 * time.Hour, "ticketRegex": "CATS"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "CATS"}`,
			expectedSuccess: true,
		},
		{
			name:            "Test Long Silences With Custom Ticket Work",
			decider:         testutil.MustMakeDecider(t, deciders.TemplateFunc(silenceshaveticket.New), map[string]interface{}{"maxLength": 8 * time.Hour, "ticketRegex": "CATS1"}),
			input:           `{"startsAt":"2020-01-19T00:23:55.242Z", "endsAt":"2020-01-20T00:23:55.242Z", "comment": "CATS"}`,
			expectedSuccess: false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			response := tt.decider.Decide(testutil.MustMakeRequest(t, http.MethodGet, "/api/v2/silences", tt.input))

			if tt.expectedSuccess {
				require.Nil(t, response)
			} else {
				require.NotNil(t, response)
			}
		})
	}
}
