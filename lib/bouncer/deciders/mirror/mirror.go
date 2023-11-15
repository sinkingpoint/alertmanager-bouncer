package mirror

import (
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders"
)

// MirrorDecider is a Decider which mirrors requests that it receives to an alternate location. This can be used for e.g. to spin up testing
// alertmanagers which receive everything a production one does.
type MirrorDecider struct {
	Destination string `mapstructure:"destination"`
}

func New(config map[string]interface{}) (deciders.Decider, error) {
	var decider MirrorDecider
	err := mapstructure.Decode(config, &decider)
	if err != nil {
		return nil, err
	}

	return &decider, nil
}

// Decide implements deciders.Decider.
func (m *MirrorDecider) Decide(req *http.Request) *deciders.HTTPError {
	url := m.Destination + req.RequestURI
	request, err := http.NewRequest(
		req.Method,
		url,
		req.Body,
	)

	if err != nil {
		return &deciders.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    fmt.Sprintf("failed to create request to %s: %s", url, err),
		}
	}

	for name, values := range req.Header {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return &deciders.HTTPError{
			Status: http.StatusBadGateway,
			Err:    fmt.Sprintf("failed to mirror request to %s: %s", url, err),
		}
	}

	resp.Body.Close()

	return nil
}
