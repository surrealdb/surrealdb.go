package connection

import (
	"context"
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
