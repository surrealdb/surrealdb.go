package main

import (
	"context"
	"encoding/json"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

//nolint:funlen
func main() {
	// Connect to SurrealDB
	db, err := surrealdb.New("ws://localhost:8000")
	if err != nil {
		panic(err)
	}

	// Set the namespace and database
	if err = db.Use(context.Background(), "testNS", "testDB"); err != nil {
		panic(err)
	}

	// Sign in to authentication `db`
	authData := &surrealdb.Auth{
		Username: "root", // use your setup username
		Password: "root", // use your setup password
	}
	token, err := db.SignIn(context.Background(), authData)
	if err != nil {
		panic(err)
	}

	// Check token validity.
	// This is not necessary if you called `SignIn` before.
	// This authenticates the `db` instance too if sign in was
	// not previously called
	if err = db.Authenticate(context.Background(), token); err != nil {
		panic(err)
	}

	// And we can later on invalidate the token if desired
	defer func(_ string) {
		if err = db.Invalidate(context.Background()); err != nil {
			panic(err)
		}
	}(token)

	// Create an entry
	person1, err := surrealdb.Create[Person](context.Background(), db, models.Table("persons"), map[interface{}]interface{}{
		"Name":     "John",
		"Surname":  "Doe",
		"Location": models.NewGeometryPoint(-0.11, 22.00),
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created person with a map: %+v\n", person1)

	// Or use structs
	person2, err := surrealdb.Create[Person](context.Background(), db, models.Table("persons"), Person{
		Name:     "John",
		Surname:  "Doe",
		Location: models.NewGeometryPoint(-0.11, 22.00),
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created person with a struvt: %+v\n", person2)

	// Get entry by Record ID
	person, err := surrealdb.Select[PersonWithCustomID, models.RecordID](context.Background(), db, *person1.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected a person by record id: %+v\n", person)
	personInJSON, err := json.Marshal(person)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected a person by record id (in JSON with custom ID JSON encoder): %s\n", string(personInJSON))

	// Or retrieve the entire table
	persons, err := surrealdb.Select[[]Person, models.Table](context.Background(), db, models.Table("persons"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected all in persons table: %+v\n", persons)

	// Delete an entry by ID
	if _, err = surrealdb.Delete[Person](context.Background(), db, *person2.ID); err != nil {
		panic(err)
	}

	// Delete all entries
	if _, err = surrealdb.Delete[[]Person](context.Background(), db, models.Table("persons")); err != nil {
		panic(err)
	}

	// Confirm empty table
	persons, err = surrealdb.Select[[]Person](context.Background(), db, models.Table("persons"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("No Selected person: %+v\n", persons)
}
