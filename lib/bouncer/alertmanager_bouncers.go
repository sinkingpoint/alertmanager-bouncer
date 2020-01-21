package bouncer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type matcher struct {
	IsRegex bool   `json:"isRegex"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

// alertmanagerSilenceSerialized represents a serialized silence from amtool in JSON
// e.g.
// {
//   "comment": "test",
//   "createdBy": "colin@quirl.co.nz",
//   "endsAt": "2020-01-13T16:34:49.444Z",
//   "matchers": [
//     {
//       "isRegex": false,
//       "name": "cats",
//       "value": "cats"
//     }
//   ],
//   "startsAt": "2020-01-13T15:34:49.444Z"
// }
type alertmanagerSilenceSerialized struct {
	Comment  string    `json:"comment"`
	Author   string    `json:"createdBy"`
	StartsAt string    `json:"startsAt"`
	EndsAt   string    `json:"endsAt"`
	Matchers []matcher `json:"matchers"`
}

// AlertmanagerSilence represents a Silence to be applied to Alertmanager
type AlertmanagerSilence struct {
	Comment  string
	Author   string
	StartsAt time.Time
	EndsAt   time.Time
	Matchers []matcher
}

func parseAlertmanagerSilence(body io.ReadCloser) (AlertmanagerSilence, error) {
	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		return AlertmanagerSilence{}, fmt.Errorf("Failed to read body")
	}
	serialized := alertmanagerSilenceSerialized{}
	json.Unmarshal(bodyBytes, &serialized)
	startTime, err := time.Parse(time.RFC3339, serialized.StartsAt)
	if err != nil {
		return AlertmanagerSilence{}, fmt.Errorf("Start Time %s is not a valid RFC3339 time string", serialized.StartsAt)
	}
	endTime, err := time.Parse(time.RFC3339, serialized.EndsAt)
	if err != nil {
		return AlertmanagerSilence{}, fmt.Errorf("End Time %s is not a valid RFC3339 time string", serialized.EndsAt)
	}

	return AlertmanagerSilence{
		Comment:  serialized.Comment,
		Author:   serialized.Author,
		StartsAt: startTime,
		EndsAt:   endTime,
		Matchers: serialized.Matchers,
	}, nil
}

// AllSilencesHaveAuthorDecider returns a decider that rejects requests that
// do not have authors which end in the given domain string
func AllSilencesHaveAuthorDecider(domain string) Decider {
	return func(req *http.Request) *HTTPError {
		silence, err := parseAlertmanagerSilence(req.Body)
		if err != nil {
			return &HTTPError{
				Status: 400,
				Err:    err,
			}
		}

		if !strings.HasSuffix(silence.Author, domain) {
			return &HTTPError{
				Status: 400,
				Err:    fmt.Errorf("Authors must be %s emails. Got %s", domain, silence.Author),
			}
		}

		return nil
	}
}
