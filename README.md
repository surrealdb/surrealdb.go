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

[//]: # (In the example below you can see how to connect to a remote instance of SurrealDB, authenticating with the database, and issuing queries for creating, updating, and selecting data from records.)
In the example provided below, we are going to connect and authenticate on a SurrealDB server, set the namespace and make several data manipulation requests.
> This example requires SurrealDB to be [installed](https://surrealdb.com/install) and running on port 8000.

```go
package main

import (
	surrealdb "github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

type User struct {
	ID      	models.RecordID `json:"id,omitempty"`
	Name    	string `json:"name"`
	Surname 	string `json:"surname"`
	Location 	models.GeometryPoint `json:"location"`
}

func main() {
	// Connect to SurrealDB
	db, err := surrealdb.New("ws://localhost:8000")
	if err != nil {
		panic(err)
	}

	// Set the namespace and database
	if err = db.Use("testNS", "testDB"); err != nil {
		panic(err)
	}

	// Alternatively you can set the namespace and database during authentication
	authData := surrealdb.Auth{
		Database:  "testDB",
		Namespace: "testNS",
		Username:  "root",
		Password:  "password",
	}
	token, err := db.SignIn(authData)
	if err != nil {
		panic(err)
	}
	
	// Check token validity. This also authenticates the `db` if sign in was 
	// not previously called
	if err := db.Authenticate(token); err != nil {
		panic(err)
	}
	
	// And we can later on invalidate the token if desired
	defer func(token string) {
		if err := db.Invalidate(); err != nil {
			panic(err)
		}
	}(token)

	// Define user struct
	user := User{
		Name:     "John",
		Surname:  "Doe",
		Location: models.NewGeometryPoint(-0.11, 22.00),
	}

	// Insert user
	user, err := surrealdb.Create[User](db, "user", surrealdb.H{
		Name:     "John",
		Surname:  "Doe",
		Location: models.NewGeometryPoint(-0.11, 22.00),
	})
	if err != nil {
		panic(err)
	}
	
	// Get user by ID
	users, err := surrealdb.Select[User, models.RecordID](db, user.ID)
	if err != nil {
		panic(err)
	}
	
	// Delete user by ID
	if err = surrealdb.Delete[models.RecordID](db, user.ID); err != nil {
		panic(err)
	}
}
```
### Doing it your way
All Data manipulation methods are handled by an undelying `send` function. This function is 
exposed via `db.Send` function if you want to create requests yourself but limited to a selected set of methods. Theses
methods are:
- select
- create
- insert
- upsert
- update
- patch
- delete
- query
```go
type UserSelectResult struct {
	Result []Users
}

var res UserSelectResult
// or var res surrealdb.Result[[]Users]

users, err := db.Send(&res, "query", user.ID)
if err != nil {
	panic(err)
}
	
```

### Instructions for running the example

- In a new folder, create a file called `main.go` and paste the above code
- Run `go mod init github.com/<github-username>/<project-name>` to initialise a `go.mod` file
- Run `go mod tidy` to download the `surrealdb.go` dependency
- Run `go run main.go` to run the example.

## Connection Engines
There are 2 different connection engines you can use to connect to SurrealDb backend. You can do so via Websocket or through HTTP
connections

### Via Websocket
```go
db, err := surrealdb.New("ws://localhost:8000")
```
or for a secure connection
```go
db, err := surrealdb.New("wss://localhost:8000")
```

### Via HTTP
There are some functions that are not available on RPC when using HTTP but on Websocket. All these except
the "live" endpoint are effectively implemented in the HTTP library and provides the same result as though
it is natively available on HTTP. While using the HTTP connection engine, note that live queries will still
use a websocket connection if the backend supports it
```go
db, err := surrealdb.New("http://localhost:8000")
```
or for a secure connection
```go
db, err := surrealdb.New("https://localhost:8000")
```


## Data Models
This package facilitates communication between client and the backend service using the Concise 
Binary Object Representation (CBOR) format. It streamlines data serialization and deserialization 
while ensuring efficient and lightweight communication. The library also provides custom models 
tailored to specific Data models recognised by SurrealDb, which cannot be covered by idiomatic go, enabling seamless interaction between 
the client and the backend.

See the [documetation on data models](https://surrealdb.com/docs/surrealql/datamodel) on support data types

| CBOR Type         |  Go Representation | Example                    |
|-------------------|-----------------------------|----------------------------|
| Null              | `nil`                       | `var x interface{} = nil`  |
| None              | `surrealdb.None`           | `map[string]interface{}{"customer": surrealdb.None}`  |
| Boolean           | `bool`                      | `true`, `false`            |
| Array             | `[]interface{}`             | `[]MyStruct{item1, item2}`  |
| Date/Time         | `time.Time`                 | `time.Now()`               |
| Duration         | `time.Duration`              | `time.Duration(8821356)`        |
| UUID (string representation)  | `surrealdb.UUID(string)` | `surrealdb.UUID("123e4567-e89b-12d3-a456-426614174000")` |
| UUID (binary representation)  | `surrealdb.UUIDBin([]bytes)`| `surrealdb.UUIDBin([]byte{0x01, 0x02, ...}`)` |
| Integer  | `uint`, `uint64`,  `int`, `int64`            | `42`, `uint64(100000)`,  `-42`, `int64(-100000)`  |
| Floating Point    | `float32`, `float64`         | `3.14`, `float64(2.71828)` |
| Byte String, Binary Encoded Data       | `[]byte`                    | `[]byte{0x01, 0x02}`       |
| Text String | `string`            | `"Hello, World!"`          |
| Map   | `map[interface{}]interface{}`   | `map[string]float64{"one": 1.0}` |
| Table name| `surrealdb.Table(name)`   | `surrealdb.Table("users")`          |
| Record ID| `surrealdb.RecordID{Table: string, ID: interface{}}`   | `surrealdb.RecordID{Table: "customers", ID: 1}, surrealdb.NewRecordID("customers", 1)`          |
| Geometry Point | `surrealdb.GeometryPoint{Latitude: float64, Longitude: float64}`                    | `surrealdb.GeometryPoint{Latitude: 11.11, Longitude: 22.22`          |
| Geometry Line | `surrealdb.GeometryLine{GeometricPoint1, GeometricPoint2,... }`                    |       |
| Geometry Polygon | `surrealdb.GeometryPolygon{GeometryLine1, GeometryLine2,... }`                    |       |
| Geometry Multipoint | `surrealdb.GeometryMultiPoint{GeometryPoint1, GeometryPoint2,... }`   |       |
| Geometry MultiLine | `surrealdb.GeometryMultiLine{GeometryLine1, GeometryLine2,... }`   |       |
| Geometry MultiPolygon | `surrealdb.GeometryMultiPolygon{GeometryPolygon1, GeometryPolygon2,... }`   |       |
| Geometry Collection| `surrealdb.GeometryMultiPolygon{GeometryPolygon1, GeometryLine2, GeometryPoint3, GeometryMultiPoint4,... }`   |       |




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




