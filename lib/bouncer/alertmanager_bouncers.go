package bouncer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type deciderTemplate struct {
	requiredConfigVars []string
	templateFunc       func(map[string]string) Decider
}

var deciderTemplates map[string]deciderTemplate

// InitDeciderTemplates sets up the bouncerTemplates map
// allowing finding a bouncerTemplate by a given name. Used to
// deserialize a list of bouncers
func InitDeciderTemplates() {
	deciderTemplates = map[string]deciderTemplate{
		"AllSilencesHaveAuthor": {
			requiredConfigVars: []string{"domain"},
			templateFunc:       AllSilencesHaveAuthorDecider,
		},
		"Mirror": {
			requiredConfigVars: []string{"destination"},
			templateFunc:       MirrorDecider,
		},
		"SilencesDontExpireOnWeekends": {
			requiredConfigVars: []string{},
			templateFunc:       SilencesDontExpireOnWeekendsDecider,
		},
	}
}

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
func AllSilencesHaveAuthorDecider(config map[string]string) Decider {
	return func(req *http.Request) *HTTPError {
		domain := config["domain"]
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

// MirrorDecider is a Decider which mirrors requests that it receives
// to an alternate location. This can be used for e.g. to spin up testing
// alertmanagers which receive everything a production one does
func MirrorDecider(config map[string]string) Decider {
	destination := config["destination"]
	return func(req *http.Request) *HTTPError {
		url := destination + req.RequestURI
		request, err := http.NewRequest(
			req.Method,
			url,
			req.Body,
		)

		if err != nil {
			return &HTTPError{
				Status: 500,
				Err:    fmt.Errorf("Failed to create request to %s: %s", url, err),
			}
		}

		for name, values := range req.Header {
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}

		log.Printf("Mirroring request to %s\n", destination)

		http.DefaultClient.Do(request)
		return nil
	}
}

// SilencesDontExpireOnWeekendsDecider returns a Decider which rejects silences
// that expire on weekends, so that we don't spring any surprises on someone oncall over the weekend
func SilencesDontExpireOnWeekendsDecider(config map[string]string) Decider {
	return func(req *http.Request) *HTTPError {
		silence, err := parseAlertmanagerSilence(req.Body)
		if err != nil {
			return &HTTPError{
				Status: 400,
				Err:    err,
			}
		}

		weekday := silence.EndsAt.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return &HTTPError{
				Status: 400,
				Err:    fmt.Errorf("By policy, silences can't expire on weekends. Be nice to people oncall over the weekend! "),
			}
		}

		return nil
	}
}
