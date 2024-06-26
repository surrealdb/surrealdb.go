<br>

<p align="center">
    <img width=120 src="https://raw.githubusercontent.com/surrealdb/icons/main/surreal.svg" />
    &nbsp;
    <img width=120 src="https://raw.githubusercontent.com/surrealdb/icons/main/golang.svg" />
</p>

<h3 align="center">The official SurrealDB SDK for Golang.</h3>

<br>

<p align="center">
    <a href="https://github.com/surrealdb/surrealdb.go"><img src="https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://surrealdb.com/docs/integration/libraries/golang"><img src="https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://pkg.go.dev/github.com/surrealdb/surrealdb.go"><img src="https://img.shields.io/github/go-mod/go-version/surrealdb/surrealdb.go?style=flat-square&label=go"></a>
	&nbsp;
	<a href="https://goreportcard.com/report/github.com/surrealdb/surrealdb.go"><img src="https://goreportcard.com/badge/github.com/surrealdb/surrealdb.go?style=flat-square"></a>
</p>

<p align="center">
    <a href="https://surrealdb.com/discord"><img src="https://img.shields.io/discord/902568124350599239?label=discord&style=flat-square&color=5a66f6"></a>
    &nbsp;
    <a href="https://twitter.com/surrealdb"><img src="https://img.shields.io/badge/twitter-follow_us-1d9bf0.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://www.linkedin.com/company/surrealdb/"><img src="https://img.shields.io/badge/linkedin-connect_with_us-0a66c2.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://www.youtube.com/channel/UCjf2teVEuYVvvVC-gFZNq6w"><img src="https://img.shields.io/badge/youtube-subscribe-fc1c1c.svg?style=flat-square"></a>
</p>

# surrealdb.go

The official SurrealDB SDK for Golang.

## Documentation

View the SDK documentation [here](https://surrealdb.com/docs/integration/libraries/golang).

## How to install

```sh
go get github.com/surrealdb/surrealdb.go
```

## Getting started

In the example below you can see how to connect to a remote instance of SurrealDB, authenticating with the database, and issuing queries for creating, updating, and selecting data from records.

> This example requires SurrealDB to be [installed](https://surrealdb.com/install) and running on port 8000.

```go
package main

import (
	"github.com/surrealdb/surrealdb.go"
)

type User struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func main() {
	// Connect to SurrealDB
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}

	authData := &surrealdb.Auth{
		Database:  "test",
		Namespace: "test",
		Username:  "root",
		Password:  "root",
	}
	if _, err = db.Signin(authData); err != nil {
		panic(err)
	}

	if _, err = db.Use("test", "test"); err != nil {
		panic(err)
	}

	// Define user struct
	user := User{
		Name:    "John",
		Surname: "Doe",
	}

	// Insert user
	data, err := db.Create("user", user)
	if err != nil {
		panic(err)
	}

	// Unmarshal data
	createdUser := make([]User, 1)
	err = surrealdb.Unmarshal(data, &createdUser)
	if err != nil {
		panic(err)
	}

	// Get user by ID
	data, err = db.Select(createdUser[0].ID)
	if err != nil {
		panic(err)
	}

	// Unmarshal data
	selectedUser := new(User)
	err = surrealdb.Unmarshal(data, &selectedUser)
	if err != nil {
		panic(err)
	}

	// Change part/parts of user
	changes := map[string]string{"name": "Jane"}

	// Update user
	if _, err = db.Update(selectedUser.ID, changes); err != nil {
		panic(err)
	}

	if _, err = db.Query("SELECT * FROM $record", map[string]interface{}{
		"record": createdUser[0].ID,
	}); err != nil {
		panic(err)
	}

	// Delete user by ID
	if _, err = db.Delete(selectedUser.ID); err != nil {
		panic(err)
	}
}
```

### Instructions for running the example

- In a new folder, create a file called `main.go` and paste the above code
- Run `go mod init github.com/<github-username>/<project-name>` to initialise a `go.mod` file
- Run `go mod tidy` to download the `surrealdb.go` dependency
- Run `go run main.go` to run the example.

## Contributing

You can run the Makefile commands to run and build the project

```
make build
make test
make lint
```

You also need to be running SurrealDB alongside the tests.
We recommend using the nightly build, as development may rely on the latest functionality.

## Helper functions

### Smart Marshal

SurrealDB Go library supports smart marshal. It means that you can use any type of data as a value in your struct, and the library will automatically convert it to the correct type.

```go
// User struct is a test struct
user, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Create, user[0]))

// Can be used without SmartUnmarshal
data, err := surrealdb.SmartMarshal(s.db.Create, user[0])
```

### Smart Unmarshal

SurrealDB Go library supports smart unmarshal. It means that you can unmarshal any type of data to the generic type provided, and the library will automatically convert it to that type.

```go
// User struct is a test struct
data, err := surrealdb.SmartUnmarshal[testUser](s.db.Select(user[0].ID))
```




