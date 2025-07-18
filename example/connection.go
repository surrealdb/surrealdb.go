package main

import (
	"os"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

const (
	defaultURL = "ws://localhost:8000"
)

var currentURL = os.Getenv("SURREALDB_URL")

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
	db, err := surrealdb.New(getSurrealDBWSURL())
	if err != nil {
		panic(err)
	}

	return initConnection(db, "examples", database, tables...)
}

func newSurrealDBHTTPConnection(database string, tables ...string) *surrealdb.DB {
	db, err := surrealdb.New(getSurrealDBHTTPURL())
	if err != nil {
		panic(err)
	}

	return initConnection(db, "examples", database, tables...)
}

func initConnection(db *surrealdb.DB, namespace, database string, tables ...string) *surrealdb.DB {
	var err error

	if err = db.Use(namespace, database); err != nil {
		panic(err)
	}

	authData := &map[string]any{
		"username": "root",
		"password": "root",
	}
	token, err := db.SignIn(authData)
	if err != nil {
		panic(err)
	}

	if err = db.Authenticate(token); err != nil {
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
		if _, err = surrealdb.Query[[]any](db, "REMOVE TABLE IF EXISTS "+table, nil); err != nil {
			panic(err)
		}
	}

	return db
}
