# surrealdb.go

The official SurrealDB library for Golang.

[![](https://img.shields.io/badge/status-beta-ff00bb.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go) [![](https://img.shields.io/badge/docs-view-44cc11.svg?style=flat-square)](https://surrealdb.com/docs/integration/libraries/golang) [![](https://img.shields.io/badge/license-Apache_License_2.0-00bfff.svg?style=flat-square)](https://github.com/surrealdb/surrealdb.go)

## How To Use The Client For The First Time

Using https://surrealdb.com/install, install SurrealDB on your computer

Once installed, run the following command:
```surreal start --log trace --user root --pass root memory```

The above command will start up a SurrealDB instance on localhost with the ability to sign in with the following credentials:
* username: root
* password: root

Then go to the ```/examples/``` directory and run ```main.go```

That's it!

And remember, if you are using an IDE such as Intellij, you can also debug the code to gain an even better understanding of how the client is working.
