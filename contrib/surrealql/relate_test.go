package surrealql

import "testing"

func TestRelate(t *testing.T) {
	tests := []struct {
		name      string
		query     Query
		wantSurQL string
	}{
		{
			name:      "simple relate",
			query:     Relate("users:123", "likes", "posts:456"),
			wantSurQL: "RELATE users:123->likes->posts:456",
		},
		{
			name:      "relate with set",
			query:     Relate("users:123", "likes", "posts:456").Set("rating", 5),
			wantSurQL: "RELATE users:123->likes->posts:456 SET rating = $param_1",
		},
		{
			name:      "relate with content",
			query:     Relate("users:123", "likes", "posts:456").Content(map[string]any{"rating": 5, "timestamp": "2024-01-01"}),
			wantSurQL: "RELATE users:123->likes->posts:456 CONTENT $content_1",
		},
		{
			name:      "relate with compound operation",
			query:     Relate("users:123", "views", "posts:456").Set("count += ?", 1).Set("last_viewed", "2024-01-01"),
			wantSurQL: "RELATE users:123->views->posts:456 SET count += $param_1, last_viewed = $param_2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSurQL, _ := tt.query.Build()

			if gotSurQL != tt.wantSurQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", gotSurQL, tt.wantSurQL)
			}
		})
	}
}
