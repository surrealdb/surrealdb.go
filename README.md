# surrealdb.go

The official SurrealDB library for Golang.

[![](https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go) 
[![](https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square)](https://surrealdb.com/docs/integration/libraries/golang)
[![Go Reference](https://pkg.go.dev/badge/github.com/surrealdb/surrealdb.go.svg)](https://pkg.go.dev/github.com/surrealdb/surrealdb.go)
[![Go Report Card](https://goreportcard.com/badge/github.com/surrealdb/surrealdb.go)](https://goreportcard.com/report/github.com/surrealdb/surrealdb.go)
[![](https://img.shields.io/badge/license-Apache_License_2.0-00bfff.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

## Getting Started

### Installation

```bash
go get github.com/surrealdb/surrealdb.go
```

### Usage

```go
package main

import (
    "fmt"
    "github.com/surrealdb/surrealdb.go"
)

func main() {
	// Connect to SurrealDB
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}

	// Sign in
	if _, err = db.Signin(map[string]string{
		"user": "root",
		"pass": "root",
	}); err != nil {
		panic(err)
	}

	// Select namespace and database
	if _, err = db.Use("test", "test"); err != nil {
		panic(err)
	}

	// Create user struct
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
	if _, err = db.Change(selectedUser.ID, changes); err != nil {
		panic(err)
	}

	// Update user
	if _, err = db.Update(selectedUser.ID, changes); err != nil {
		panic(err)
	}

	// Raw Query user
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

# Documentation

Full documentation is available at [surrealdb doc](https://surrealdb.com/docs/integration/libraries/golang)


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
data, err := surrealdb.SmartUnmarshal[testUser](s.db.Select(user[0].ID))

```




