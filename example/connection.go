package main

import (
	"context"
	"os"
	"strings"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
)

const (
	defaultURL = "ws://localhost:8000"
)

var currentURL = os.Getenv("SURREALDB_URL")
var reconnect = os.Getenv("SURREALDB_RECONNECTION_CHECK_INTERVAL")

func getSurrealDBWSURL() string {
	if currentURL == "" {
		return defaultURL
	}
	return currentURL
}

func getSurrealDBHTTPURL() string {
	if currentURL == "" {
		return "http://localhost:8000"
	}
	return strings.ReplaceAll(currentURL, "ws", "http")
}

func newSurrealDBWSConnection(database string, tables ...string) *surrealdb.DB {
	var reconnectDuration time.Duration
	if reconnect != "" {
		var err error
		reconnectDuration, err = time.ParseDuration(reconnect)
		if err != nil {
			panic("Invalid SURREALDB_RECONNECTION_CHECK_INTERVAL: " + reconnect)
		}
	}

	var (
		db  *surrealdb.DB
		err error
	)

	if os.Getenv("SURREALDB_CONNECTION_IMPL") == "gws" {
		p, err := surrealdb.Configure(getSurrealDBWSURL(),
			surrealdb.WithReconnectionCheckInterval(reconnectDuration),
		)
		if err != nil {
			panic(err)
		}
		g := gws.New(*p)
		if err = g.Connect(context.Background()); err != nil {
			panic(err)
		}
		db = surrealdb.New(g)
	} else {
		db, err = surrealdb.Connect(
			context.Background(),
			getSurrealDBWSURL(),
			surrealdb.WithReconnectionCheckInterval(reconnectDuration),
		)
		if err != nil {
			panic(err)
		}
	}

	return initConnection(db, "examples", database, tables...)
}

func newSurrealDBHTTPConnection(database string, tables ...string) *surrealdb.DB {
	db, err := surrealdb.Connect(
		context.Background(),
		getSurrealDBHTTPURL(),
	)
	if err != nil {
		panic(err)
	}

	return initConnection(db, "examples", database, tables...)
}

func initConnection(db *surrealdb.DB, namespace, database string, tables ...string) *surrealdb.DB {
	var err error

	if err = db.Use(context.Background(), namespace, database); err != nil {
		panic(err)
	}

	authData := &surrealdb.Auth{
		Username: "root",
		Password: "root",
	}
	token, err := db.SignIn(context.Background(), authData)
	if err != nil {
		panic(err)
	}

	if err = db.Authenticate(context.Background(), token); err != nil {
		panic(err)
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
			panic(err)
		}
	}

	return db
}
