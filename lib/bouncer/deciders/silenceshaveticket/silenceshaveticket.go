package silenceshaveticket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/regexp"
	"github.com/mitchellh/mapstructure"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
)

type SilencesHaveTicket struct {
	MaxLength   time.Duration `mapstructure:"maxLength"`
	TicketRegex string        `mapstructure:"ticketRegex"`

	ticketRegex *regexp.Regexp
}

func New(config map[string]interface{}) (deciders.Decider, error) {
	var decider SilencesHaveTicket
	if err := mapstructure.Decode(config, &decider); err != nil {
		return nil, err
	}

	if decider.MaxLength == 0 {
		return nil, fmt.Errorf("maxLength must be set")
	}

	if decider.TicketRegex == "" {
		decider.ticketRegex = regexp.MustCompile("^[A-Z]+-[0-9]+")
	} else {
		regex, err := regexp.Compile(decider.TicketRegex)
		if err != nil {
			return nil, fmt.Errorf("failed to compile ticketRegex: %s", err)
		}
		decider.ticketRegex = regex
	}

	return &decider, nil
}

// LongSilencesHaveTicketDecider returns a Decider which rejects silences longer than
// the given duration, which don't have a comment matching the "ticket_regex" (defaults to a JIRA ticket format)
// This allows us to not have long running throwaway silences without a ticket to track ongoing work.
func (a *SilencesHaveTicket) Decide(req *http.Request) *deciders.HTTPError {
	silence, err := deciders.ParseSilence(req.Body)
	if err != nil {
		return &deciders.HTTPError{
			Status: http.StatusBadRequest,
			Err:    err.Error(),
		}
	}

	tooLong := silence.EndsAt.Sub(silence.StartsAt) > a.MaxLength
	hasTicket := a.ticketRegex.MatchString(silence.Comment)

	if tooLong && !hasTicket {
		return &deciders.HTTPError{
			Status: http.StatusBadRequest,
			Err:    fmt.Sprintf("silences longer than %s must have tickets attached to them to track ongoing work", a.MaxLength),
		}
	}

	return nil
}
