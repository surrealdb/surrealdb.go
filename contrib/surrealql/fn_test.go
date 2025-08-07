package surrealql

import "testing"

func TestFn(t *testing.T) {
	tests := []struct {
		name   string
		fc     *FunCall
		wantQL string
	}{
		{
			name:   "sum",
			fc:     Fn("math::sum").ArgFromField("amount"),
			wantQL: "math::sum(amount)",
		},
		{
			name:   "avg",
			fc:     Fn("math::mean").ArgFromField("price"),
			wantQL: "math::mean(price)",
		},
		{
			name:   "min",
			fc:     Fn("math::min").ArgFromField("created_at"),
			wantQL: "math::min(created_at)",
		},
		{
			name:   "max",
			fc:     Fn("math::max").ArgFromField("score"),
			wantQL: "math::max(score)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getQL, _ := tt.fc.Build()

			if getQL != tt.wantQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", getQL, tt.wantQL)
			}
		})
	}
}
