package connection

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"ws://h:8000":            "http://h:8000",
		"ws://h/rpc":             "http://h",
		"ws://h/rpc/":            "http://h",
		"wss://h/surreal":        "https://h/surreal",
		"wss://h/surreal/rpc":    "https://h/surreal",
		"https://h/surreal/?x=1": "https://h/surreal",
		"http://h/":              "http://h",
	} {
		got, err := HTTPBaseURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := HTTPBaseURL("ftp://h")
	assert.Error(t, err)
}

// TestCheckHealth_keepsThePathPrefix checks that a server behind a reverse
// proxy's path prefix is asked for /health under that prefix, as it is for /rpc.
func TestCheckHealth_keepsThePathPrefix(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if r.URL.Path != "/surreal/health" {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	require.NoError(t, CheckHealth(context.Background(), nil, server.URL+"/surreal"))
	require.NoError(t, CheckHealth(context.Background(), nil, "ws"+server.URL[len("http"):]+"/surreal/rpc"))
	assert.Equal(t, []string{"/surreal/health", "/surreal/health"}, asked)
}

// TestCheckHealth_unhealthy checks that an answer other than 2xx is a
// *HealthError holding the status and the body, and that a server which cannot
// be reached is not.
func TestCheckHealth_unhealthy(t *testing.T) {
	answer := func(status int, body string) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(server.Close)
		return server
	}

	err := CheckHealth(context.Background(), nil, answer(http.StatusServiceUnavailable, " datastore unavailable\n").URL)
	var he *HealthError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusServiceUnavailable, he.StatusCode)
	assert.Equal(t, "503 Service Unavailable", he.Status)
	assert.Equal(t, "datastore unavailable", he.Body)
	assert.EqualError(t, err, "health check failed: 503 Service Unavailable: datastore unavailable")

	err = CheckHealth(context.Background(), nil, answer(http.StatusInternalServerError, "").URL)
	require.ErrorAs(t, err, &he)
	assert.Empty(t, he.Body)
	assert.EqualError(t, err, "health check failed: 500 Internal Server Error")

	unreachable := answer(http.StatusOK, "")
	unreachable.Close()
	err = CheckHealth(context.Background(), nil, unreachable.URL)
	require.Error(t, err)
	assert.False(t, errors.As(err, &he), "a server which cannot be reached gives no status")
}
