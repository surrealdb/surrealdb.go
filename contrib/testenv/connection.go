// Package testenv provides utilities for testing the SurrealDB Go SDK
// and SurrealDB.
//
// It includes functions to create connections to SurrealDB instances
// over WebSocket and HTTP, as well as helper functions for testing purposes.
package testenv

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

const (
	// DefaultWSURL is the default WebSocket URL for SurrealDB.
	DefaultWSURL = "ws://localhost:8000"

	// EnvWSURL is the environment variable that specifies the SurrealDB WebSocket URL.
	// If not set, it defaults to DefaultWSURL.
	EnvWSURL = "SURREALDB_URL"

	// EnvReconnectionCheckInterval is the environment variable that specifies the
	// reconnection check interval for WebSocket connections.
	EnvReconnectionCheckInterval = "SURREALDB_RECONNECTION_CHECK_INTERVAL"

	// EnvSurrealDBConnectionImpl is the environment variable that specifies
	// the SurrealDB connection implementation to use.
	// If set to "gws", it uses the gws package; otherwise, it
	// defaults to the gorillaws package.
	EnvSurrealDBConnectionImpl = "SURREALDB_CONNECTION_IMPL"

	// EnvSurrealCBORImpl is the environment variable that specifies
	// the SurrealDB CBOR implementation to use.
	EnvSurrealCBORImpl = "SURREALDB_CBOR_IMPL"
)

// CBORImpl specifies which CBOR implementation to use
type CBORImpl int

const (
	// CBORImplDefault uses the default implementation from connection.NewConfig (currently surrealcbor)
	CBORImplDefault CBORImpl = iota
	// CBORImplSurrealCBOR explicitly uses the surrealcbor implementation
	CBORImplSurrealCBOR
	// CBORImplFxamackerCBOR explicitly uses the fxamacker/cbor implementation
	CBORImplFxamackerCBOR
)

// String returns the string representation of CBORImpl
func (c CBORImpl) String() string {
	switch c {
	case CBORImplDefault:
		return "default"
	case CBORImplSurrealCBOR:
		return "surrealcbor"
	case CBORImplFxamackerCBOR:
		return "fxamackercbor"
	default:
		return "unknown"
	}
}

var (
	currentURL     = os.Getenv(EnvWSURL)
	reconnect      = os.Getenv(EnvReconnectionCheckInterval)
	useGWS         = os.Getenv(EnvSurrealDBConnectionImpl) == "gws"
	cborImplEnvVar = os.Getenv(EnvSurrealCBORImpl)

	// defaultCBORImpl determines the default CBOR implementation based on environment variable
	defaultCBORImpl = func() CBORImpl {
		switch cborImplEnvVar {
		case "surrealcbor":
			return CBORImplSurrealCBOR
		case "fxamackercbor":
			return CBORImplFxamackerCBOR
		default:
			return CBORImplDefault
		}
	}()
)

func GetSurrealDBURL() string {
	if currentURL == "" {
		return DefaultWSURL
	}
	return currentURL
}

func MustParseSurrealDBURL() *url.URL {
	u, err := url.Parse(GetSurrealDBURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse SurrealDB URL: %v", err))
	}
	return u
}

func MustParseSurrealDBWSURL() *url.URL {
	u, err := url.Parse(GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse SurrealDB WebSocket URL: %v", err))
	}
	return u
}

func getSurrealDBHTTPURL() string {
	if currentURL == "" {
		return "http://localhost:8000"
	}
	return strings.ReplaceAll(currentURL, "ws", "http")
}

func GetSurrealDBWSURL() string {
	if currentURL == "" {
		return DefaultWSURL
	}
	return strings.ReplaceAll(currentURL, "http", "ws")
}

type Config struct {
	// Endpoint is the SurrealDB endpoint URL.
	Endpoint string

	Namespace string
	Database  string
	Tables    []string

	// ReconnectDuration is the duration to wait before attempting to reconnect
	// to the SurrealDB instance after a disconnection.
	ReconnectDuration time.Duration

	// CBORImpl specifies which CBOR implementation to use.
	// Default is CBORImplDefault which uses the default from connection.NewConfig.
	CBORImpl CBORImpl
}

func MustNew(namespace, database string, tables ...string) *surrealdb.DB {
	db, err := New(namespace, database, tables...)
	if err != nil {
		panic(fmt.Sprintf("Failed to create SurrealDB connection: %v", err))
	}
	return db
}

// New creates a new SurrealDB connection with the specified database and tables.
// The connection information is derived from environment variables.
// It supports both WebSocket and HTTP connections based on the URL scheme.
func New(namespace, database string, tables ...string) (*surrealdb.DB, error) {
	c, err := NewConfig(namespace, database, tables...)
	if err != nil {
		return nil, err
	}

	return c.New()
}

func MustNewConfig(namespace, database string, tables ...string) *Config {
	c, err := NewConfig(namespace, database, tables...)
	if err != nil {
		panic(err)
	}
	return c
}

func NewConfig(namespace, database string, tables ...string) (*Config, error) {
	var reconnectDuration time.Duration
	if reconnect != "" {
		var err error
		reconnectDuration, err = time.ParseDuration(reconnect)
		if err != nil {
			return nil, fmt.Errorf("invalid SURREALDB_RECONNECTION_CHECK_INTERVAL: %s", reconnect)
		}
	}

	c := &Config{
		Endpoint:          GetSurrealDBURL(),
		Namespace:         namespace,
		Database:          database,
		Tables:            tables,
		ReconnectDuration: reconnectDuration,
		CBORImpl:          defaultCBORImpl,
	}

	return c, nil
}

func (c *Config) MustNew() *surrealdb.DB {
	db, err := c.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create SurrealDB connection: %v", err))
	}
	return db
}

func (c *Config) New() (*surrealdb.DB, error) {
	if c.Database == "" {
		return nil, fmt.Errorf("database name must be specified")
	}

	if len(c.Tables) == 0 {
		return nil, fmt.Errorf("at least one table name must be specified")
	}

	u, err := url.ParseRequestURI(c.Endpoint)
	if err != nil {
		return nil, err
	}

	conf := connection.NewConfig(u)
	switch c.CBORImpl {
	case CBORImplFxamackerCBOR:
		// Explicitly use fxamacker/cbor implementation
		conf.Marshaler = &models.CborMarshaler{}     //nolint:staticcheck // Intentional use of deprecated type for legacy support
		conf.Unmarshaler = &models.CborUnmarshaler{} //nolint:staticcheck // Intentional use of deprecated type for legacy support
	case CBORImplSurrealCBOR:
		// Explicitly use surrealcbor implementation
		codec := surrealcbor.New()
		conf.Marshaler = codec
		conf.Unmarshaler = codec
	case CBORImplDefault:
		// Use the default from connection.NewConfig (which is surrealcbor)
	}

	var conn connection.Connection
	if c.ReconnectDuration > 0 {
		switch conf.URL.Scheme {
		case "ws", "wss":
			if useGWS {
				conn = rews.New(
					func(ctx context.Context) (*gws.Connection, error) {
						return gws.New(conf), nil
					},
					c.ReconnectDuration,
					conf.Unmarshaler,
					conf.Logger,
				)
			} else {
				conn = rews.New(
					func(ctx context.Context) (*gorillaws.Connection, error) {
						return gorillaws.New(conf), nil
					},
					c.ReconnectDuration,
					conf.Unmarshaler,
					conf.Logger,
				)
			}
		case "http", "https":
			return nil, fmt.Errorf("reconnection is not supported for HTTP connections")
		default:
			return nil, fmt.Errorf("invalid connection URL scheme: %s", conf.URL.Scheme)
		}
	} else {
		switch conf.URL.Scheme {
		case "http", "https":
			conn = http.New(conf)
		case "ws", "wss":
			if useGWS {
				conn = gws.New(conf)
			} else {
				conn = gorillaws.New(conf)
			}
		default:
			return nil, fmt.Errorf("invalid connection URL scheme: %s", conf.URL.Scheme)
		}
	}

	db, err := surrealdb.FromConnection(context.Background(), conn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
	}

	return Init(db, c.Namespace, c.Database, c.Tables...)
}

func MustNewHTTP(database string, tables ...string) *surrealdb.DB {
	db, err := NewHTTP(database, tables...)
	if err != nil {
		panic(err)
	}
	return db
}

func NewHTTP(database string, tables ...string) (*surrealdb.DB, error) {
	db, err := surrealdb.FromEndpointURLString(
		context.Background(),
		getSurrealDBHTTPURL(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SurrealDB HTTP endpoint: %w", err)
	}

	return Init(db, "examples", database, tables...)
}

// Init initializes the testing environment.
// It cleans up the specified tables in the namespace/database.
// If no tables are specified, it will clean up all tables in the database.
func Init(db *surrealdb.DB, namespace, database string, tables ...string) (*surrealdb.DB, error) {
	var err error

	if err = db.Use(context.Background(), namespace, database); err != nil {
		return nil, fmt.Errorf("failed to use database: %w", err)
	}

	authData := &surrealdb.Auth{
		Username: "root",
		Password: "root",
	}
	token, err := db.SignIn(context.Background(), authData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign in: %w", err)
	}

	if err = db.Authenticate(context.Background(), token); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	// If no tables specified, get all tables in the database
	if len(tables) == 0 {
		query := "INFO FOR DB"
		if result, infoErr := surrealdb.Query[map[string]any](context.Background(), db, query, nil); infoErr == nil && len(*result) > 0 {
			if info, ok := (*result)[0].Result["tables"].(map[string]any); ok {
				for tableName := range info {
					tables = append(tables, tableName)
				}
			}
		}
		// If we couldn't get tables or there are no tables, that's fine - nothing to clean
	}

	// Clean up everything in the specified database
	for _, table := range tables {
		// Note that each of the below queries will fail in their own way:
		//
		// - REMOVE TABLE IF EXISTS type::table($tb) will fail with:
		//
		//     There was a problem with the database: Parse error: Unexpected token `::`, expected Eof
		//     REMOVE TABLE IF EXISTS type::table($tb)
		//                                ^^
		//
		// - REMOVE TABLE IF EXISTS $tb will fail with:
		//
		//     There was a problem with the database: Parse error: Unexpected token `a parameter`, expected an identifier
		//     REMOVE TABLE IF EXISTS $tb
		//							  ^^
		if _, err = surrealdb.Query[[]any](context.Background(), db, "REMOVE TABLE IF EXISTS "+table, nil); err != nil {
			return nil, fmt.Errorf("failed to remove table %s: %w", table, err)
		}
	}

	return db, nil
}
