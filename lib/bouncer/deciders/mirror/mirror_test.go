package mirror_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/deciders/mirror"
	"github.com/sinkingpoint/alertmanager_bouncer/lib/bouncer/testutil"
	"github.com/stretchr/testify/require"
)

func TestMirrorDecider(t *testing.T) {
	// Setup a backend server to proxy requests to
	recChan := make(chan int, 1)
	const backendResponse = "I am the backend"
	const backendStatus = http.StatusNotFound
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(backendStatus)
		w.Write([]byte(backendResponse))
		recChan <- 1
	}))
	defer backend.Close()

	decider, err := mirror.New(map[string]interface{}{"destination": backend.URL})
	require.NoError(t, err)
	require.Nil(t, decider.Decide(testutil.MustMakeRequest(t, http.MethodGet, "/api/v1/silences", "")))

	select {
	case <-recChan:
	case <-time.After(500 * time.Millisecond):
		require.FailNow(t, "Expected request to be mirrored to the backend, but it wasn't")
	}
}
