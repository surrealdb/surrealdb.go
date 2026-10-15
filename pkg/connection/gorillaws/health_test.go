package gorillaws

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
