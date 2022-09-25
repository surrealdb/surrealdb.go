package surrealdb_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/test-go/testify/require"
)

func Test_QueryResolver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, surrealdb.GetEnvOrDefault("SURREALDB_RPC_URL", "ws://localhost:8000/rpc"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     surrealdb.GetEnvOrDefault("SURREALDB_USER", "root"),
		Password: surrealdb.GetEnvOrDefault("SURREALDB_PASS", "root"),
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	type johnny struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	result := surrealdb.Query[johnny](db, ctx, "update users:johnny set Username = 'johnny', Password = 'secret'")
	require.NoError(t, result.Error())
	first := result.First()
	require.Equal(t, "johnny", first.Username)
	require.Equal(t, "secret", first.Password)

	print(result, first)
}
