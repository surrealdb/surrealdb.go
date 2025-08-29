
## Getting started with SurrealDB SDK for Go

To connect to SurrealDB and perform data operations, please refer to the complete example in [main.go](cmd/main.go).

> This example requires SurrealDB to be [installed](https://surrealdb.com/install) and running on port 8000.

### Running the Example

The easiest way to get started is to run the example directly:

```sh
# Clone the repository
git clone https://github.com/surrealdb/surrealdb.go.git
cd surrealdb.go

# Run the example
go run ./example/cmd
```

The `example` directory contains a complete working example demonstrating basic CRUD operations.

### Testable Examples

The testable example files [`example*_test.go`](..) demonstrate various SDK features including:
- Query operations and transactions
- Relations and graph traversal
- Bulk operations (insert, upsert)
- Authentication methods
- Custom CBOR configuration
- And many more use cases

You can run any of the example tests with:
```sh
# Clone the repository
git clone https://github.com/surrealdb/surrealdb.go.git
cd surrealdb.go

# Run the testable example
go test -v ./ -run ExampleName
```

If you're viewing this documentation at `pkg.go.dev`, you can view the examples alongside corresponding SDK functions.
