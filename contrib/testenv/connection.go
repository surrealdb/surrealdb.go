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
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
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
)

var (
	currentURL = os.Getenv(EnvWSURL)
	reconnect  = os.Getenv(EnvReconnectionCheckInterval)
	useGWS     = os.Getenv(EnvSurrealDBConnectionImpl) == "gws"
)

func getSurrealDBURL() string {
	if currentURL == "" {
		return DefaultWSURL
	}
	return currentURL
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

func MustNewDeprecated(database string, tables ...string) *surrealdb.DB {
	db, err := New("examples", database, tables...)
	if err != nil {
		panic(fmt.Sprintf("Failed to create SurrealDB connection: %v", err))
	}
	return db
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
	if database == "" {
		return nil, fmt.Errorf("database name must be specified")
	}

	if len(tables) == 0 {
		return nil, fmt.Errorf("at least one table name must be specified")
	}

	var reconnectDuration time.Duration
	if reconnect != "" {
		var err error
		reconnectDuration, err = time.ParseDuration(reconnect)
		if err != nil {
			return nil, fmt.Errorf("invalid SURREALDB_RECONNECTION_CHECK_INTERVAL: %s", reconnect)
		}
	}

	var connect func(ctx context.Context) (*surrealdb.DB, error)

	if reconnectDuration > 0 {
		u, err := url.ParseRequestURI(getSurrealDBURL())
		if err != nil {
			return nil, err
		}

		conf := connection.NewConfig(u)

		switch conf.URL.Scheme {
		case "ws", "wss":
			if useGWS {
				connect = func(ctx context.Context) (*surrealdb.DB, error) {
					gwsConn := gws.New(conf)

					return surrealdb.FromConnection(ctx, gwsConn)
				}
			} else {
				connect = func(ctx context.Context) (*surrealdb.DB, error) {
					wsConn := gorillaws.New(conf)

					return surrealdb.FromConnection(ctx, wsConn)
				}
			}
		case "http", "https":
			return nil, fmt.Errorf("reconnection is not supported for HTTP connections")
		default:
			return nil, fmt.Errorf("invalid connection URL scheme: %s", conf.URL.Scheme)
		}
	} else {
		connect = func(ctx context.Context) (*surrealdb.DB, error) {
			httpConn, err := surrealdb.FromEndpointURLString(ctx, getSurrealDBHTTPURL())
			if err != nil {
				return nil, err
			}
			return httpConn, nil
		}
	}

	db, err := connect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
	}

	return initConnection(db, namespace, database, tables...)
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

	return initConnection(db, "examples", database, tables...)
}

func initConnection(db *surrealdb.DB, namespace, database string, tables ...string) (*surrealdb.DB, error) {
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
