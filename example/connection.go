package main

import (
	"os"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const (
	defaultURL = "ws://localhost:8000"
)

var currentURL = os.Getenv("SURREALDB_URL")

func getSurrealDBURL() string {
	if currentURL == "" {
		return defaultURL
	}
	return currentURL
}

func newSurrealDBConnection(namespace, database string, tables ...string) *surrealdb.DB {
	db, err := surrealdb.New(getSurrealDBURL())
	if err != nil {
		panic(err)
	}

	if err = db.Use(namespace, database); err != nil {
		panic(err)
	}

	authData := &surrealdb.Auth{
		Username: "root",
		Password: "root",
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
		if _, err = surrealdb.Delete[[]any](db, models.Table(table)); err != nil {
			panic(err)
		}
	}

	return db
}
