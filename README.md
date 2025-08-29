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

The official SurrealDB SDK for Go.

## Documentation

View the SDK documentation on [pkg.go.dev](https://pkg.go.dev/github.com/surrealdb/surrealdb.go) or [here](https://surrealdb.com/docs/integration/libraries/golang).

## How to install

```sh
go get github.com/surrealdb/surrealdb.go
```

## Getting started

To connect to SurrealDB and perform data operations, please refer to the complete example in [./example/main.go](./example/main.go).

> This example requires SurrealDB to be [installed](https://surrealdb.com/install) and running on port 8000.

### Running the Example

The easiest way to get started is to run the example directly:

```sh
# Clone the repository
git clone https://github.com/surrealdb/surrealdb.go.git
cd surrealdb.go

# Run the example
go run ./example
```

The `./example` directory contains a complete working example demonstrating basic CRUD operations.

### Testable Examples

Testable example files (`example*_test.go`) demonstrate various SDK features including:
- Query operations and transactions
- Relations and graph traversal
- Bulk operations (insert, upsert)
- Authentication methods
- Custom CBOR configuration
- And many more use cases

You can run any of the example tests with:
```sh
go test -v ./ -run ExampleName
```

If you're viewing this documentation at `pkg.go.dev`, you can view the examples alongside corresponding SDK functions.

## Executing SurrealQL

### Using Query

For most use cases, you can use the `Query` function to execute SurrealQL statements. This is the recommended approach for complex queries, transactions, and when you need full control over your database operations:

```go
// Execute a SurrealQL query with typed results
results, err := surrealdb.Query[[]Person](
    context.Background(),
    db,
    "SELECT * FROM persons WHERE age > $minAge",
    map[string]any{
        "minAge": 18,
    },
)

// You can also use Query for transactions with variables
transactionResults, err := surrealdb.Query[[]any](
    context.Background(),
    db,
    `
    BEGIN TRANSACTION;
    CREATE person:$johnId SET name = $johnName, age = $johnAge;
    CREATE person:$janeId SET name = $janeName, age = $janeAge;
    COMMIT TRANSACTION;
    `,
    map[string]any{
        "johnId": "john",
        "johnName": "John",
        "johnAge": 30,
        "janeId": "jane",
        "janeName": "Jane",
        "janeAge": 25,
    },
)

// Or use a single CREATE with content variable
createResult, err := surrealdb.Query[[]Person](
    context.Background(),
    db,
    "CREATE person:$id CONTENT $content",
    map[string]any{
        "id": "alice",
        "content": map[string]any{
            "name": "Alice",
            "age": 28,
            "city": "New York",
        },
    },
)
```

The `Query` function supports:
- Full SurrealQL syntax including transactions
- Parameterized queries for security
- Typed results with generics
- Multiple statements in a single call

### Using Send for low-level control

All data manipulation methods are handled by an underlying `send` function. This function is
exposed via `db.Send` function if you want to create requests yourself but limited to a selected set of methods. These
methods are:

-   select
-   create
-   insert
-   upsert
-   update
-   patch
-   delete
-   query

```go
type UserSelectResult struct {
	Result []Users
}

var res UserSelectResult
// or var res surrealdb.Result[[]Users]

err := db.Send(context.Background(), &res, "select", user.ID)
if err != nil {
	panic(err)
}
```

## Connection Engines

There are 2 different connection engines you can use to connect to SurrealDb backend. You can do so via Websocket or through HTTP
connections

### Via Websocket

```go
db, err := surrealdb.FromEndpointURLString(ctx, "ws://localhost:8000")
```

or for a secure connection

```go
db, err := surrealdb.FromEndpointURLString(ctx, "wss://localhost:8000")
```

### Via HTTP

There are some functions that are not available on RPC when using HTTP but on Websocket. All these except
the "live" endpoint are effectively implemented in the HTTP library and provides the same result as though
it is natively available on HTTP. While using the HTTP connection engine, note that live queries will still
use a websocket connection if the backend supports it

```go
db, err := surrealdb.FromEndpointURLString(ctx, "http://localhost:8000")
```

or for a secure connection

```go
db, err := surrealdb.FromEndpointURLString(ctx, "https://localhost:8000")
```

### Using SurrealKV and Memory

SurrealKV and Memory also do not support live notifications at this time. This would be updated in the next
release.

> ⚠️ **Note**
> Although the examples below reference `surrealkv://` and `memory://`, **these modes are currently not supported in the Go SDK**.
> Support for embedded databases (like SurrealKV and in-memory) is pending future development and is being tracked in [issue #197](https://github.com/surrealdb/surrealdb.go/issues/197).
>
> Until then, please use `http://` or `ws://` endpoints to connect to a running SurrealDB server instance.

For Surreal KV

```go
db, err := surrealdb.New("surrealkv://path/to/dbfile.kv")
```

For Memory

```go
db, err := surrealdb.New("mem://")
db, err := surrealdb.New("memory://")
```

## Data Models

This package facilitates communication between client and the backend service using the Concise
Binary Object Representation (CBOR) format. It streamlines data serialization and deserialization
while ensuring efficient and lightweight communication. The library also provides custom models
tailored to specific Data models recognised by SurrealDb, which cannot be covered by idiomatic go, enabling seamless interaction between
the client and the backend.

See the [documetation on data models](https://surrealdb.com/docs/surrealql/datamodel) on support data types

| CBOR Type                        | Go Representation                                                                                           | Example                                                                                |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| Null                             | `nil`                                                                                                       | `var x any = nil`                                                                      |
| None                             | `surrealdb.None`                                                                                            | `map[string]any{"customer": surrealdb.None}`                                           |
| Boolean                          | `bool`                                                                                                      | `true`, `false`                                                                        |
| Array                            | `[]any`                                                                                                     | `[]MyStruct{item1, item2}`                                                             |
| Date/Time                        | `time.Time`                                                                                                 | `time.Now()`                                                                           |
| Duration                         | `time.Duration`                                                                                             | `time.Duration(8821356)`                                                               |
| UUID (string representation)     | `surrealdb.UUID(string)`                                                                                    | `surrealdb.UUID("123e4567-e89b-12d3-a456-426614174000")`                               |
| UUID (binary representation)     | `surrealdb.UUIDBin([]bytes)`                                                                                | `surrealdb.UUIDBin([]byte{0x01, 0x02, ...}`)`                                          |
| Integer                          | `uint`, `uint64`, `int`, `int64`                                                                            | `42`, `uint64(100000)`, `-42`, `int64(-100000)`                                        |
| Floating Point                   | `float32`, `float64`                                                                                        | `3.14`, `float64(2.71828)`                                                             |
| Byte String, Binary Encoded Data | `[]byte`                                                                                                    | `[]byte{0x01, 0x02}`                                                                   |
| Text String                      | `string`                                                                                                    | `"Hello, World!"`                                                                      |
| Map                              | `map[any]any`                                                                                               | `map[string]float64{"one": 1.0}`                                                       |
| Table name                       | `surrealdb.Table(name)`                                                                                     | `surrealdb.Table("users")`                                                             |
| Record ID                        | `surrealdb.RecordID{Table: string, ID: any}`                                                                | `surrealdb.RecordID{Table: "customers", ID: 1}, surrealdb.NewRecordID("customers", 1)` |
| Geometry Point                   | `surrealdb.GeometryPoint{Latitude: float64, Longitude: float64}`                                            | `surrealdb.GeometryPoint{Latitude: 11.11, Longitude: 22.22`                            |
| Geometry Line                    | `surrealdb.GeometryLine{GeometricPoint1, GeometricPoint2,... }`                                             |                                                                                        |
| Geometry Polygon                 | `surrealdb.GeometryPolygon{GeometryLine1, GeometryLine2,... }`                                              |                                                                                        |
| Geometry Multipoint              | `surrealdb.GeometryMultiPoint{GeometryPoint1, GeometryPoint2,... }`                                         |                                                                                        |
| Geometry MultiLine               | `surrealdb.GeometryMultiLine{GeometryLine1, GeometryLine2,... }`                                            |                                                                                        |
| Geometry MultiPolygon            | `surrealdb.GeometryMultiPolygon{GeometryPolygon1, GeometryPolygon2,... }`                                   |                                                                                        |
| Geometry Collection              | `surrealdb.GeometryMultiPolygon{GeometryPolygon1, GeometryLine2, GeometryPoint3, GeometryMultiPoint4,... }` |                                                                                        |

## Helper Types

### surrealdb.O

For some methods like create, insert, update, you can pass a map instead of an struct value. An example:

```go
person, err := surrealdb.Create[Person](context.Background(), db, models.Table("persons"), map[any]any{
	"Name":     "John",
	"Surname":  "Doe",
	"Location": models.NewGeometryPoint(-0.11, 22.00),
})
```

This can be simplified to:

```go
person, err := surrealdb.Create[Person](context.Background(), db, models.Table("persons"), surrealdb.O{
	"Name":     "John",
	"Surname":  "Doe",
	"Location": models.NewGeometryPoint(-0.11, 22.00),
})
```

Where surrealdb.O is defined below. There is no special advantage in using this other than simplicity/legibility.

```go
type surrealdb.O map[any]any
```

### surrealdb.Result[T]

This is useful for the `Send` function where `T` is the expected response type for a request. An example:

```go
var res surrealdb.Result[[]Users]
err := db.Send(context.Background(), &res, "select", model.Table("users"))
if err != nil {
	panic(err)
}
fmt.Printf("users: %+v\n", users.R)
```

## Contributing

You can run the Makefile commands to run and build the project:

```shell
make lint test
```

You also need to be running SurrealDB alongside the tests.
We recommend using the nightly build, as development may rely on the latest functionality.
