package cborwire

import (
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// benchRows encodes 1,000 rows. With rewrite set, each row holds a datetime,
// a UUID and a duration as text tags, which Normalize rewrites.
func benchRows(b *testing.B, rewrite bool) []byte {
	b.Helper()
	rows := make([]map[string]any, 1000)
	for i := range rows {
		row := map[string]any{"name": fmt.Sprintf("row %d", i), "n": i, "score": 1.5}
		if rewrite {
			row["at"] = cbor.Tag{Number: 0, Content: "2026-01-02T03:04:05.123456789Z"}
			row["id"] = cbor.Tag{Number: 9, Content: "0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"}
			row["dur"] = cbor.Tag{Number: 13, Content: "1h30m5s"}
		}
		rows[i] = row
	}
	data, err := cbor.Marshal(rows)
	if err != nil {
		b.Fatal(err)
	}
	return data
}

func BenchmarkNormalize(b *testing.B) {
	for _, rewrite := range []bool{false, true} {
		b.Run(fmt.Sprintf("rewrite=%v", rewrite), func(b *testing.B) {
			data := benchRows(b, rewrite)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for range b.N {
				if _, err := Normalize(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
