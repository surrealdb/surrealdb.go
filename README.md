<br>

<p align="center">
    <img width=120 src="https://raw.githubusercontent.com/surrealdb/icons/main/surreal.svg" />
    &nbsp;
    <img width=120 src="https://raw.githubusercontent.com/surrealdb/icons/main/golang.svg" />
</p>

<h3 align="center">The official SurrealDB SDK for Go</h3>

<br>

<p align="center">
    <a href="https://github.com/surrealdb/surrealdb.go"><img src="https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://surrealdb.com/docs/integration/libraries/golang"><img src="https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://pkg.go.dev/github.com/surrealdb/surrealdb.go/v2"><img src="https://img.shields.io/github/go-mod/go-version/surrealdb/surrealdb.go?style=flat-square&label=go"></a>
	&nbsp;
	<a href="https://goreportcard.com/report/github.com/surrealdb/surrealdb.go/v2"><img src="https://goreportcard.com/badge/github.com/surrealdb/surrealdb.go/v2?style=flat-square"></a>
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

## How to install

```sh
go get github.com/surrealdb/surrealdb.go/v2
```

## "Getting started" example

To connect to SurrealDB and perform data operations, please refer to [example](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/v2/example#section-readme).

## Using the SDK

To use the SDK, please refer to the [documentation](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/v2#section-documentation).

### Supported SurrealDB versions

The SDK supports SurrealDB 3.3 and later 3.x versions, and 4.x versions,
including pre-releases such as 4.0.0-beta.1. `surrealdb.MinimumServerVersion` and
`surrealdb.MaximumServerVersion` give the range. To make connecting fail when
the server is outside it, pass `surrealdb.WithVersionCheck(true)`. The check is
off by default.

SurrealDB 4.0 is recommended. With a 3.x server, these limits apply:

- The kinds that SurrealDB 4.0 adds, such as the sized numbers (`i8` to `u64`,
  `f16` and `f32`), need a 4.0 server. A 3.x server does not speak cbor.2, so
  the SDK uses plain CBOR, and sized numbers are sent as `int` and `float`.
- `QueryRaw` cannot always match results to statements when the query has a
  transaction. SurrealDB 3.x does not return one result for each statement
  when a transaction has a `RETURN` statement, or is canceled after a
  statement in it failed. `QueryRaw` then returns an error, and gives no
  statement a result. Use `Query` for these transactions.
- Error kinds are less precise. For example, a value that does not match a
  field's type is an `Internal` error with code -32000 on 3.x, but a
  `Validation` error with code -32600 on 4.0. Do not depend on
  `ServerError.Kind` or `ServerError.Details` to have the 4.0 values.

### SurrealDB 4.0 and cbor.2

SurrealDB 4.0 adds cbor.2, a version of the CBOR protocol that keeps its sized
numbers (`i8` to `u64`, `f16` and `f32`). The SDK negotiates it on its own and
falls back to plain CBOR with servers before 4.0:

- WebSocket connections offer the subprotocols `cbor.2` and `cbor`, and use
  whichever the server picks.
- HTTP connections ask for `application/cbor; v=2` responses, and send cbor.2
  requests once the server has answered in it.

Over cbor.2, Go's `int8`, `uint8`, `int16`, `uint16`, `int32`, `uint32` and
`float32` values are sent as the matching sized kinds, `uint64` values above
the `int64` range as `u64`, and `models.Float16` as `f16`; decoding into `any`
gives back those Go types. `db.CBORVersion()` says which version a connection
negotiated, and `connection.Config.DisableCBORv2` turns cbor.2 off. The
deprecated `models.CborMarshaler` and `models.CborUnmarshaler` only speak
plain CBOR. See the [`surrealcbor.Codec`](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/v2/surrealcbor#Codec)
documentation for the details.

#### Dates, times, timestamps, intervals, bigdecimals and regexes

SurrealDB 4.0 also adds kinds that it shares with PostgreSQL and MySQL. The
`models` package has a type for each, named as in the JavaScript SDK:

| SurrealQL | Go type | Holds |
|---|---|---|
| `date` | `models.PlainDate` | a calendar date with no time or zone |
| `time` | `models.PlainTime` | a time of day with no date or zone, up to 24:00:00 |
| `timestamp` | `models.Timestamp` | a wall-clock date and time with no zone, which is not an instant |
| `interval` | `models.Interval` | months, days and microseconds, kept apart |
| `bigdecimal` | `models.BigDecimal` | a decimal past the 28 digits of a `decimal`, as its text |
| `regex` | `models.Regex` | the text of a pattern, in the syntax of the Rust `regex` crate |

```go
due := models.MustParsePlainDate("2024-02-29")
res, err := surrealdb.Query[[]models.PlainDate](ctx, db, "RETURN [$due, pd'infinity']", map[string]any{"due": due})
```

Over cbor.2 each kind keeps its type both ways, and decoding into `any` gives
these types. Into another Go type a value decodes as plain CBOR would carry
it: a date into a `time.Time` is its midnight UTC. The zero value of each type
is a real value, such as 1970-01-01, not NONE: for an optional field, use a
pointer with `omitempty`. The SDK never compiles a `models.Regex`, as the
syntax of Go's `regexp` differs from Rust's.

Plain CBOR has none of these kinds. Over it, the server and the SDK send a
date or a timestamp as a datetime, a time as a duration, an interval as its
ISO 8601 text, and a bigdecimal as a decimal when it fits one and as text
otherwise. A regex cannot be sent: encoding one fails with an error that wraps
`constants.ErrNeedsCBOR2`.

An HTTP connection sends plain CBOR until the server has answered it in
cbor.2, and again after any answer that is not cbor.2, such as an error page.
A value of these kinds in such a request is stored as its widened kind. A
regex fails before anything is sent, and as no answer comes back, it keeps
failing until another request switches the connection to cbor.2. `Let` over
HTTP keeps a plain CBOR copy
of each variable, so it cannot hold a regex. To send cbor.2 from the first
request, connect with `surrealdb.WithVersionCheck(true)`, which asks the
server for its version on connect, or sign in first.

## Contributing

You can run the Makefile commands to run and build the project:

```shell
make lint test
```

You also need to be running SurrealDB alongside the tests.
We recommend using the nightly build, as development may rely on the latest functionality.

The CBOR codec is also tested against the shared conformance vectors in
[surrealdb-protocol](https://github.com/surrealdb/surrealdb-protocol). Those
tests skip unless you check the vectors out first:

```shell
eval "$(scripts/fetch-surrealdb-protocol.sh)"
go test ./surrealcbor -run TestConformanceVectors
```

## Links

<!--
This section is intentionally named "Links" so that the links
are displayed in the right sidecbar.

https://github.com/golang/go/issues/42968
-->

- [The SDK documentation on pkg.go.dev](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/v2)
- [The SDK documentation on surrealdb.com](https://surrealdb.com/docs/integration/libraries/golang)
