package benchmark_test

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"testing"

	surrealdb "github.com/surrealdb/surrealdb.go/v2"
)

// a simple user struct for testing
type testUser struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	ID       string `json:"id,omitempty"`
}

func SetupMockDB() (*surrealdb.DB, error) {
	return surrealdb.New("")
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
		surrealdb.Create[testUser](db, models.Table("users"), users[i]) //nolint:errcheck
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
		surrealdb.Select[testUser](db, models.NewRecordID("users", "bob")) //nolint:errcheck
	}
}
