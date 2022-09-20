<br>

<p align="center">
    <a href="https://surrealdb.com#gh-dark-mode-only" target="_blank">
        <img width="300" src="/img/white/logo_go.svg" alt="SurrealDB GO Logo">
    </a>
    <a href="https://surrealdb.com#gh-light-mode-only" target="_blank">
        <img width="300" src="/img/black/logo_go.svg" alt="SurrealDB GO Logo">
    </a>
</p>

<h3 align="center">
    The official
    <a href="https://surrealdb.com#gh-dark-mode-only" target="_blank">
        <img src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/white/text.svg" height="15" alt="SurrealDB">
    </a>
    <a href="https://surrealdb.com#gh-light-mode-only" target="_blank">
        <img src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/black/text.svg" height="15" alt="SurrealDB">
    </a>
    library for GO.
</h3>

<br>

<p align="center">
    <a href="https://github.com/surrealdb/surrealdb.go"><img src="https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://surrealdb.com/docs/integration/libraries/golang"><img src="https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square"></a>
    &nbsp;
    <a href="https://pkg.go.dev/github.com/surrealdb/surrealdb.go"><img src="https://pkg.go.dev/badge/github.com/surrealdb/surrealdb.go.svg"></a>
    &nbsp;
    <a href="https://github.com/surrealdb/license"><img src="https://img.shields.io/badge/license-Apache_License_2.0-00bfff.svg?style=flat-square"></a>
</p>

<p align="center">
	<a href="https://surrealdb.com/blog"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/blog.svg" alt="Blog"></a>
	&nbsp;
	<a href="https://github.com/surrealdb/surrealdb"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/github.svg" alt="Github	"></a>
	&nbsp;
    <a href="https://www.linkedin.com/company/surrealdb/"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/linkedin.svg" alt="LinkedIn"></a>
    &nbsp;
    <a href="https://twitter.com/surrealdb"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/twitter.svg" alt="Twitter"></a>
    &nbsp;
    <a href="https://www.youtube.com/channel/UCjf2teVEuYVvvVC-gFZNq6w"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/youtube.svg" alt="Youtube"></a>
    &nbsp;
    <a href="https://dev.to/surrealdb"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/dev.svg" alt="Dev"></a>
    &nbsp;
    <a href="https://surrealdb.com/discord"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/discord.svg" alt="Discord"></a>
    &nbsp;
    <a href="https://stackoverflow.com/questions/tagged/surrealdb"><img height="25" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/social/stack-overflow.svg" alt="StackOverflow"></a>

</p>

<br>

<h2><img height="20" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/whatissurreal.svg">&nbsp;&nbsp;Looking for the core code?</h2>

Built in `Rust` the core source code for
<a href="https://surrealdb.com#gh-dark-mode-only" target="_blank">
<img src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/white/text.svg" height="15" alt="SurrealDB">
</a>
<a href="https://surrealdb.com#gh-light-mode-only" target="_blank">
<img src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/black/text.svg" height="15" alt="SurrealDB">
</a>
can be found [here](https://github.com/surrealdb/surrealdb).

<br>

<h2><img height="20" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/documentation.svg">&nbsp;&nbsp;Documentation</h2>

The complete and detailed documentation for this library is located [here](https://surrealdb.com/docs/integration/libraries/golang).

<br>

<h2><img height="20" src="https://raw.githubusercontent.com/surrealdb/surrealdb/main/img/gettingstarted.svg">&nbsp;&nbsp;Quick start</h2>

First we install the library with `go get`
```cli
go get github.com/surrealdb/surrealdb.go
```

Once you have installed it, you can now import to your project
```go
import (
	"github.com/surrealdb/surrealdb.go"
)
```

Connecting to a database.
```go
db, err := surrealdb.New("ws://localhost:8000/rpc")
if err != nil {
    panic(err)
}
```

Authenticating...
```go
_, err = db.Signin(map[string]interface{}{
    "user": "root",
    "pass": "root",
})
if err != nil {
    panic(err)
}
```

Then we need to specify in which `Namespace` and `Database` we intend to operate on.
```go
_, err = db.Use("test", "test")
if err != nil {
    panic(err)
}
```