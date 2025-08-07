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
			name:      "relate with content",
			query:     Relate("users:123", "likes", "posts:456").Set("rating", 5),
			wantSurQL: "RELATE users:123->likes->posts:456 CONTENT $content_1",
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
