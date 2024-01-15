# surrealdb.go

The official SurrealDB library for Golang.

[![](https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go) 
[![](https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square)](https://surrealdb.com/docs/integration/libraries/golang)
[![Go Reference](https://pkg.go.dev/badge/github.com/surrealdb/surrealdb.go.svg)](https://pkg.go.dev/github.com/surrealdb/surrealdb.go)
[![Go Report Card](https://goreportcard.com/badge/github.com/surrealdb/surrealdb.go)](https://goreportcard.com/report/github.com/surrealdb/surrealdb.go)
[![](https://img.shields.io/badge/license-Apache_License_2.0-00bfff.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

## Getting Started

For instructions on how to follow SurrealDB, follow [Installation Guide](https://surrealdb.com/docs/installation)

### Installation

```bash
go get github.com/surrealdb/surrealdb.go
```

### Usage

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
	db, err := surrealdb.New(context.Background(), "ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}

	authData := &surrealdb.Auth{
		Database:  "test",
		Namespace: "test",
		Username:  "root",
		Password:  "root",
	}
	if _, err = db.Signin(context.Background(), authData); err != nil {
		panic(err)
	}

	if _, err = db.Use(context.Background(), "test", "test"); err != nil {
		panic(err)
	}

	// Define user struct
	user := User{
		Name:    "John",
		Surname: "Doe",
	}

	// Insert user
	data, err := db.Create(context.Background(), "user", user)
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
	data, err = db.Select(context.Background(), createdUser[0].ID)
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
	if _, err = db.Update(context.Background(), selectedUser.ID, changes); err != nil {
		panic(err)
	}

	if _, err = db.Query(context.Background(), "SELECT * FROM $record", map[string]interface{}{
		"record": createdUser[0].ID,
	}); err != nil {
		panic(err)
	}

	// Delete user by ID
	if _, err = db.Delete(context.Background(), selectedUser.ID); err != nil {
		panic(err)
	}
}
```

* Step 1: Create a file called `main.go` and paste the above code
* Step 2: Run the command `go mod init github.com/<github-username>/<project-name>` to create a go.mod file
* Step 3: Run the command `go mod tidy` to download surreal db
* Step 4: Run `go run main.go` to run the application. 

# Documentation

Full documentation is available at [surrealdb doc](https://surrealdb.com/docs/integration/libraries/golang)

## Building

You can run the Makefile helper to run and build the project

```
make build
make test
make lint
```

You also need to be running SurrealDB alongside the tests.
We recommend using the nightly build, as development may rely on the latest functionality.


## Helper functions
### Smart Marshal

SurrealDB Go library supports smart marshal. It means that you can use any type of data as a value in your struct. SurrealDB Go library will automatically convert it to the correct type.

```go
// Recommended to use with SmartUnmarshal SmartMarshal
// User struct is a test struct
user, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Create, user[0]))

// Can be used without SmartUnmarshal
data, err := surrealdb.SmartMarshal(s.db.Create, user[0])
```

### Smart Unmarshal

SurrealDB Go library supports smart unmarshal. It means that you can unmarshal any type of data to the generic type provided. SurrealDB Go library will automatically convert it to that type.

```go

// User struct is a test struct
data, err := surrealdb.SmartUnmarshal[testUser](s.db.Select(context.Background(), user[0].ID))

```




