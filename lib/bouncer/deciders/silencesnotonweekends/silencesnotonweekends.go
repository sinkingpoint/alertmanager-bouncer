package silencesnotonweekends

import (
	"net/http"
	"time"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
)

type SilencesNotOnWeekends struct {
}

func New(config map[string]interface{}) (deciders.Decider, error) {
	return &SilencesNotOnWeekends{}, nil
}

// SilencesNotOnWeekendsDecider returns a decider that rejects silences that do not have authors which end in the given domain string.
func (a *SilencesNotOnWeekends) Decide(req *http.Request) *deciders.HTTPError {
	silence, err := deciders.ParseSilence(req.Body)
	if err != nil {
		return &deciders.HTTPError{
			Status: http.StatusBadRequest,
			Err:    err.Error(),
		}
	}

	weekday := silence.EndsAt.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return &deciders.HTTPError{
			Status: http.StatusBadRequest,
			Err:    "by policy, silences can't expire on weekends. Be nice to people oncall over the weekend!",
		}
	}

	return nil
}
