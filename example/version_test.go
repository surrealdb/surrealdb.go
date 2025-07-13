package main

import (
	"fmt"
)

//nolint:lll,govet
func ExampleVersion() {
	ws := newSurrealDBWSConnection("version")
	v, err := ws.Version()
	if err != nil {
		panic(err)
	}
	fmt.Printf("VersionData (WebSocket): %+v\n", v)

	http := newSurrealDBHTTPConnection("version")
	v, err = http.Version()
	if err != nil {
		panic(err)
	}
	fmt.Printf("VersionData (HTTP): %+v\n", v)

	// You get something like below depending on your SurrealDB version:
	//
	// VersionData (WebSocket): &{Version:2.3.7 Build: Timestamp:}
	// VersionData (HTTP): &{Version:2.3.7 Build: Timestamp:}
}
