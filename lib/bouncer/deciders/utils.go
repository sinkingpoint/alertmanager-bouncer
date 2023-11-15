package deciders

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/prometheus/alertmanager/types"
)

func ParseSilence(body io.ReadCloser) (types.Silence, error) {
	silence := types.Silence{}
	if err := json.NewDecoder(body).Decode(&silence); err != nil {
		return types.Silence{}, fmt.Errorf("failed to decode silence: %s", err)
	}

	return silence, nil
}
