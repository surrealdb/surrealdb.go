package main

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

//nolint:lll,govet
func ExampleDB_Version() {
	ws := testenv.MustNew("surrealdbexamples", "version")
	v, err := ws.Version(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Printf("VersionData (WebSocket): %+v\n", v)

	http := testenv.MustNew("surrealdbexamples", "version")
	v, err = http.Version(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Printf("VersionData (HTTP): %+v\n", v)

	// You get something like below depending on your SurrealDB version:
	//
	// VersionData (WebSocket): &{Version:2.3.7 Build: Timestamp:}
	// VersionData (HTTP): &{Version:2.3.7 Build: Timestamp:}
}
