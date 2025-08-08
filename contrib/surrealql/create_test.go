package surrealql

import "testing"

func TestCreate(t *testing.T) {
	tests := []struct {
		name     string
		query    Query
		wantQL   string
		wantArgs map[string]any
	}{
		{
			name:   "create with set",
			query:  Create("users").Set("name", "John").Set("email", "john@example.com"),
			wantQL: "CREATE users SET email = $email_1, name = $name_1",
			wantArgs: map[string]any{
				"name_1":  "John",
				"email_1": "john@example.com",
			},
		},
		{
			name:   "create with return none",
			query:  Create("users").Set("name", "John").ReturnNone(),
			wantQL: "CREATE users SET name = $name_1 RETURN NONE",
			wantArgs: map[string]any{
				"name_1": "John",
			},
		},
		{
			name: "create with content",
			query: Create("users").Content(map[string]any{
				"name":  "John",
				"age":   30,
				"roles": []string{"admin", "user"},
			}),
			wantQL: "CREATE users CONTENT $content_1",
			wantArgs: map[string]any{
				"content_1": map[string]any{
					"name":  "John",
					"age":   30,
					"roles": []string{"admin", "user"},
				},
			},
		},
		{
			name:   "create with compound operation",
			query:  Create("stats").Set("views", 0).Set("clicks += ?", 1),
			wantQL: "CREATE stats SET views = $views_1, clicks += $param_1",
			wantArgs: map[string]any{
				"views_1": 0,
				"param_1": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQL, gotArgs := tt.query.Build()

			if gotQL != tt.wantQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", gotQL, tt.wantQL)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("Args count mismatch\ngot:  %d\nwant: %d", len(gotArgs), len(tt.wantArgs))
			}
		})
	}
}
