package surrealdb_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
)

func setupTests(ctx context.Context, t *testing.T) *surrealdb.DB {
	db, err := surrealdb.New(ctx, getEnvOrDefault("SURREALDB_RPC_URL", "ws://0.0.0.0:8000/rpc"))
	if err != nil {
		panic(err)
	}

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     getEnvOrDefault("SURREALDB_USER", "root"),
		Password: getEnvOrDefault("SURREALDB_PASS", "root"),
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	// insert testing data

	if _, err := db.Query(ctx, "DELETE user:bob; UPDATE user:bob SET username = $username;", map[string]any{"username": "bob"}); err != nil {
		t.Errorf("Update user:bob errored: %d", err)
	}
	if _, err := db.Query(ctx, "DELETE user:bob_two; UPDATE user:bob_two SET username = $username;", map[string]any{"username": "bob"}); err != nil {
		t.Errorf("Update user:bob_two errored: %d", err)
	}
	if _, err := db.Query(ctx, "DELETE user:bob_three; UPDATE user:bob_three SET username = $username;", map[string]any{"username": "bob"}); err != nil {
		t.Errorf("Update user:bob_three errored: %d", err)
	}

	return db
}

type testUserInformation struct {
	Username string `json:"username"`
	NewValue string `json:"newValue,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Age      int    `json:"age,omitempty"`
}

func Test_QueryResolver_Query(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, t)

	result := surrealdb.Query[testUserInformation](ctx, db, "SELECT * FROM user:bob WHERE username = $username;", map[string]any{
		"username": "bob",
	})

	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}

	bob := result.First()

	if bob == nil {
		t.Errorf("Expected object for bob, got nil")
		return
	}
	if bob.Username != "bob" {
		t.Errorf("Expected bob, got %s", bob.Username)
		return
	}

}

func Test_QueryResolver_Create(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, nil)

	result := surrealdb.Create[testUserInformation](ctx, db, "user", map[string]any{
		"username": "bob",
	})
	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}
	bob := result.Item()
	if bob == nil {
		t.Errorf("Expected object for bob, got nil")
		return
	}
	if bob.Username != "bob" {
		t.Errorf("Expected bob, got %s", bob.Username)
		return
	}

}

func Test_QueryResolver_Update(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, nil)

	result := surrealdb.Update[testUserInformation](ctx, db, "user:bob_two", map[string]any{
		"newValue": "hello world",
	})
	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}
	bob := result.First()
	if bob == nil {
		t.Errorf("Expected object for bob, got nil")
		return
	}
	if bob.Username != "bob" {
		t.Errorf("Expected bob, got %s", bob.Username)
		return
	}
	if bob.NewValue != "hello world" {
		t.Errorf("Expected 'hello world', got %s", bob.NewValue)
		return
	}

}

func Test_QueryResolver_Change(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, nil)

	result := surrealdb.Change[testUserInformation](ctx, db, "user:bob_two", map[string]any{
		"newValue": "changed value",
	})
	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}
	bob := result.First()
	if bob == nil {
		t.Errorf("Expected object for bob, got nil")
		return
	}
	if bob.Username != "bob" {
		t.Errorf("Expected bob, got %s", bob.Username)
		return
	}
	if bob.NewValue != "changed value" {
		t.Errorf("Expected changed value, got %s", bob.NewValue)
		return
	}

}

func Test_QueryResolver_Modify(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, nil)

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "Bobs nickname"},
		{Op: "add", Path: "age", Value: 44},
	}

	result := surrealdb.Modify(ctx, db, "user:bob_three", patches)

	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}
	ops := result.First()
	if ops == nil || len(ops) == 0 {
		t.Errorf("Expected array of ops, got nil or empty")
		return
	}
	if len(ops) != 2 {
		t.Errorf("Expected 2 ops, got %d", len(ops))
		return
	}

	if ops[0].Op != "add" {
		t.Errorf("Expected add, got %s", ops[0].Op)
		return
	}
}

func Test_QueryResolver_Delete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupTests(ctx, nil)

	result := surrealdb.Delete[testUserInformation](ctx, db, "user:bob_three")

	if result.HasError() {
		t.Errorf("Query errored: %d", result.Error())
		return
	}

}
