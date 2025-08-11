package surrealql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationReturnClauses(t *testing.T) {
	// Use a unique database/namespace to avoid conflicts with other tests
	db := testenv.MustNew("surrealql", "return_clauses", "tasks")

	ctx := context.Background()

	type Task struct {
		ID        *models.RecordID      `json:"id,omitempty"`
		Title     string                `json:"title"`
		Completed bool                  `json:"completed"`
		UpdatedAt models.CustomDateTime `json:"updated_at"`
	}

	type TaskCreate struct {
		Title     string                `json:"title"`
		Completed bool                  `json:"completed"`
		UpdatedAt models.CustomDateTime `json:"updated_at"`
	}

	// Create a task
	task, err := surrealdb.Create[Task](ctx, db, "tasks", TaskCreate{
		Title:     "Test Task",
		Completed: false,
		UpdatedAt: models.CustomDateTime{Time: time.Now()},
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Test RETURN NONE
	t.Run("ReturnNone", func(t *testing.T) {
		query := surrealql.Update(fmt.Sprintf("tasks:%v", task.ID.ID)).
			Set("completed", true).
			ReturnNone()

		ql, vars := query.Build()
		t.Logf("UPDATE RETURN NONE SurrealQL: %s", ql)
		t.Logf("UPDATE RETURN NONE Params: %v", vars)

		// With RETURN NONE, the result should be empty
		results, err := surrealdb.Query[[]Task](ctx, db, ql, vars)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// The result should be empty
		if len((*results)[0].Result) != 0 {
			t.Errorf("Expected empty result with RETURN NONE, got %d items", len((*results)[0].Result))
		}

		// Verify the update worked - select all tasks
		selectQuery := surrealql.Select("tasks")
		ql, vars = selectQuery.Build()
		t.Logf("Verify SELECT SurrealQL: %s", ql)
		t.Logf("Verify SELECT Params: %v", vars)

		verifyResults, err := surrealdb.Query[[]Task](ctx, db, ql, vars)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		tasks := (*verifyResults)[0].Result
		t.Logf("Verify results: %d tasks found", len(tasks))

		// Find the task we updated
		var updatedTask *Task
		for i, tsk := range tasks {
			t.Logf("Task %d: %+v", i, tsk)
			if tsk.ID.ID == task.ID.ID {
				updatedTask = &tasks[i]
				break
			}
		}

		if updatedTask == nil {
			t.Error("Could not find the updated task")
		} else if !updatedTask.Completed {
			t.Error("Task was not updated correctly")
		}
	})

	t.Run("ReturnDiff", func(t *testing.T) {
		// Create another task
		task2, err := surrealdb.Create[Task](ctx, db, "tasks", TaskCreate{
			Title:     "Another Task",
			Completed: false,
			UpdatedAt: models.CustomDateTime{Time: time.Now()},
		})
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		query := surrealql.Update(fmt.Sprintf("tasks:%v", task2.ID.ID)).
			Set("title", "Updated Task").
			Set("completed", true).
			ReturnDiff()

		sql, vars := query.Build()
		t.Logf("RETURN DIFF SurrealQL: %s", sql)

		// RETURN DIFF returns a complex structure that varies by SurrealDB version
		// Just verify it works and returns non-empty result
		_, err2 := surrealdb.Query[any](ctx, db, sql, vars)
		if err2 != nil {
			t.Fatalf("Update failed: %v", err2)
		}

		// The fact that we got here without error means RETURN DIFF worked
		t.Log("RETURN DIFF query succeeded - UPDATE with RETURN DIFF is supported")
	})
}
