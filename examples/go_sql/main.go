package main

import (
	"database/sql"

	_ "github.com/surrealdb/surrealdb.go/sql"
)

func main() {
	// Connect the way you would usually
	db, err := sql.Open("surrealdb", "ws://root:root@localhost:9091/dbname")
	if err != nil {
		panic(err)
	}

	// Make sure we can ping to it
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Cleanup
	err = db.Close()
	if err != nil {
		panic(err)
	}
}
