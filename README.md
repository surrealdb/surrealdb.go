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

## How to install

```sh
go get github.com/surrealdb/surrealdb.go
```

## "Getting started" example

To connect to SurrealDB and perform data operations, please refer to [example](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/example#section-readme).

## Using the SDK

To use the SDK, please refer to the [documentation](https://pkg.go.dev/github.com/surrealdb/surrealdb.go#section-documentation).

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
plain CBOR. See the [`surrealcbor.Codec`](https://pkg.go.dev/github.com/surrealdb/surrealdb.go/surrealcbor#Codec)
documentation for the details.

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

- [The SDK documentation on pkg.go.dev](https://pkg.go.dev/github.com/surrealdb/surrealdb.go)
- [The SDK documentation on surrealdb.com](https://surrealdb.com/docs/integration/libraries/golang)
