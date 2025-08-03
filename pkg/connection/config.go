package connection

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// NewConfig creates a new Config with the SurrealDB endpoint specified by the URL.
// The URL should be a valid SurrealDB endpoint URL, such as "ws://localhost:8000/rpc" or "http://localhost:8000".
// It is not absolutely necessary to create a Config using this function,
// but it is recommended to use this function to ensure that everything needed for the connection is set up correctly.
func NewConfig(u *url.URL) *Config {
	return &Config{
		URL:         *u,
		Marshaler:   &models.CborMarshaler{},
		Unmarshaler: &models.CborUnmarshaler{},
		BaseURL:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	}
}
