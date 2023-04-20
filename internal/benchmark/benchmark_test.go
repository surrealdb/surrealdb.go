package benchmark_test

import (
	"fmt"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/internal/mock"
)

// a simple user struct for testing
type testUser struct {
	surrealdb.Basemodel `table:"test"`
	Username            string `json:"username,omitempty"`
	Password            string `json:"password,omitempty"`
	ID                  string `json:"id,omitempty"`
}

func SetupMockDB() (*surrealdb.DB, error) {
	return surrealdb.New("", mock.Create())
}

func BenchmarkCreate(b *testing.B) {
	db, err := SetupMockDB()
	if err != nil {
		b.Fatal(err)
	}
	user := &testUser{
		Username: "tobi",
		Password: "1234",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// error is ignored for benchmarking purposes.
		db.Create(fmt.Sprintf("users:%d", i), user) //nolint:errcheck
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
		db.Select("users:bob") //nolint:errcheck
	}
}
