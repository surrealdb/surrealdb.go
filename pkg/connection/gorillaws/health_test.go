package gorillaws

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// The health check trusts what the WebSocket dialer trusts, such as a private
// certificate authority.
func TestHealth_usesTheDialersTLSConfig(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	u, err := url.Parse("wss://" + strings.TrimPrefix(srv.URL, "https://"))
	require.NoError(t, err)
	con := New(connection.NewConfig(u))

	saved := DefaultDialer.TLSClientConfig
	defer func() { DefaultDialer.TLSClientConfig = saved }()

	DefaultDialer.TLSClientConfig = nil
	require.Error(t, con.Health(context.Background()), "the test server's certificate is not trusted by default")

	DefaultDialer.TLSClientConfig = &tls.Config{RootCAs: srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
	require.NoError(t, con.Health(context.Background()))
}

type wrappedTransport struct{ http.RoundTripper }

// An application may replace http.DefaultTransport with another kind of
// RoundTripper, such as one for tracing; the health check still works.
func TestHealth_defaultTransportReplaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	saved := http.DefaultTransport
	http.DefaultTransport = wrappedTransport{saved}
	defer func() { http.DefaultTransport = saved }()

	u, err := url.Parse("ws://" + strings.TrimPrefix(srv.URL, "http://"))
	require.NoError(t, err)
	require.NoError(t, New(connection.NewConfig(u)).Health(context.Background()))
}

// A TLS configuration given to http.DefaultTransport, which the WebSocket
// does not use, does not reach the health check either.
func TestHealth_ignoresTheDefaultTransportsTLSConfig(t *testing.T) {
	saved := http.DefaultTransport
	custom := saved.(*http.Transport).Clone()
	custom.TLSClientConfig = &tls.Config{ServerName: "elsewhere.invalid"}
	http.DefaultTransport = custom
	defer func() { http.DefaultTransport = saved }()

	savedTLS := DefaultDialer.TLSClientConfig
	DefaultDialer.TLSClientConfig = nil
	defer func() { DefaultDialer.TLSClientConfig = savedTLS }()

	client := httpClientFor(DefaultDialer)
	assert.Nil(t, client.Transport.(*http.Transport).TLSClientConfig)
}
