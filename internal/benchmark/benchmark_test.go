package benchmark_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/internal/mock"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

// a simple user struct for testing
type testUser struct {
	marshal.Basemodel `table:"test"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	ID                string `json:"id,omitempty"`
}

func SetupMockDB() (*surrealdb.DB, error) {
	return surrealdb.New(context.Background(), "", mock.Create())
}

func BenchmarkCreate(b *testing.B) {
	db, err := SetupMockDB()
	if err != nil {
		b.Fatal(err)
	}
	users := make([]*testUser, 0)
	for i := 0; i < b.N; i++ {
		// error is ignored for benchmarking purposes.
		users = append(users, &testUser{
			Username: "tobi",
			Password: "1234",
			ID:       fmt.Sprintf("users:%d", i),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// error is ignored for benchmarking purposes.
		db.Create(context.Background(), users[i].ID, users[i]) //nolint:errcheck
	}
}

// BenchmarkSelect benchmarks the selection of a record
func BenchmarkSelect(b *testing.B) {
	db, err := SetupMockDB()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// error is ignored for benchmarking purposes.
		db.Select(context.Background(), "users:bob") //nolint:errcheck
	}
}
