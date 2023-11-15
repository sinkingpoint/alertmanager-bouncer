package silenceshaveauthor

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
)

var _ = deciders.Decider(&SilencesHaveAuthor{})

type SilencesHaveAuthor struct {
	Domain string `mapstructure:"domain"`
}

func New(config map[string]interface{}) (deciders.Decider, error) {
	var decider SilencesHaveAuthor
	err := mapstructure.Decode(config, &decider)
	if err != nil {
		return nil, err
	}

	if decider.Domain == "" {
		return nil, fmt.Errorf("domain must be set")
	}

	return &decider, nil
}

// AllSilencesHaveAuthorDecider returns a decider that rejects silences that do not have authors which end in the given domain string.
func (a *SilencesHaveAuthor) Decide(req *http.Request) *deciders.HTTPError {
	silence, err := deciders.ParseSilence(req.Body)
	if err != nil {
		return &deciders.HTTPError{
			Status: 400,
			Err:    "failed to parse silence",
		}
	}

	if !strings.HasSuffix(silence.CreatedBy, a.Domain) {
		return &deciders.HTTPError{
			Status: 400,
			Err:    fmt.Sprintf("creators must be %q emails. Got %q", a.Domain, silence.CreatedBy),
		}
	}

	return nil
}
