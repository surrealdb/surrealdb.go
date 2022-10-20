package main

import (
	"database/sql"
	"fmt"
	sql2 "github.com/surrealdb/surrealdb.go/sql"

	_ "github.com/surrealdb/surrealdb.go/sql"
)

func main() {
	// Connect the way you would usually
	db, err := sql.Open("surrealdb", "ws://root:root@localhost:9091/?ns=my-namespace&db=dbname")
	if err != nil {
		panic(err)
	}

	// Make sure we can ping to it
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// Create some value
	_, err = db.Exec("CREATE company SET name = 'SurrealDB', cofounders = [person:tobie, person:jaime]")
	if err != nil {
		panic(err)
	}

	// Read it back
	rows, err := db.Query("SELECT cofounders, id, name FROM company")
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var (
			name       string
			id         string
			cofounders sql2.StringSlice
		)

		// Note: the columns are always sorted alphabetically, regardless of the order in your query
		err := rows.Scan(&cofounders, &id, &name)
		if err != nil {
			panic(err)
		}

		fmt.Println("row", id, name, cofounders)
	}

	// Cleanup
	err = db.Close()
	if err != nil {
		panic(err)
	}
}
