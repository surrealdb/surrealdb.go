
## Data Models

The [surrealdb] package facilitates communication between client and the backend service using the Concise
Binary Object Representation (CBOR) format.

It streamlines data serialization and deserialization
while ensuring efficient and lightweight communication.

The library also provides custom models
tailored to specific Data models recognised by SurrealDb, which cannot be covered by idiomatic Go, enabling seamless interaction between
the client and the backend.

See the [documetation on data models](https://surrealdb.com/docs/surrealql/datamodel) on data types supported in SurrealQL, and the below for ones supported in the [surrealdb] package.

[surrealdb]: https://pkg.go.dev/github.com/surrealdb/surrealdb.go

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
