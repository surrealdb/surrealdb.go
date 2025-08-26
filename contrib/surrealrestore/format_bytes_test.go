package surrealrestore

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "Zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "Less than 1KB",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "Exactly 1KB",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "1.5KB",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "Exactly 1MB",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "2.5MB",
			bytes:    int64(2.5 * 1024 * 1024),
			expected: "2.5 MB",
		},
		{
			name:     "Exactly 1GB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "1.7GB",
			bytes:    1825361101, // approximately 1.7GB
			expected: "1.7 GB",
		},
		{
			name:     "Exactly 1TB",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TB",
		},
		{
			name:     "2.3TB",
			bytes:    2528876025856, // approximately 2.3TB
			expected: "2.3 TB",
		},
		{
			name:     "Exactly 1PB",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024,
			expected: "1.0 PB",
		},
		{
			name:     "1.5PB",
			bytes:    1688849860263936, // approximately 1.5PB
			expected: "1.5 PB",
		},
		{
			name:     "Exactly 1EB",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024 * 1024,
			expected: "1.0 EB",
		},
		{
			name:     "999 bytes",
			bytes:    999,
			expected: "999 B",
		},
		{
			name:     "1023 bytes",
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "10.5KB",
			bytes:    10752,
			expected: "10.5 KB",
		},
		{
			name:     "100.1MB",
			bytes:    104962458, // approximately 100.1MB
			expected: "100.1 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s; want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestFormatBytesEdgeCases(t *testing.T) {
	// Test boundary values
	testCases := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "Just below 1KB",
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "Just above 1KB",
			bytes:    1025,
			expected: "1.0 KB",
		},
		{
			name:     "Just below 1MB",
			bytes:    1024*1024 - 1,
			expected: "1024.0 KB",
		},
		{
			name:     "Just above 1MB",
			bytes:    1024*1024 + 1,
			expected: "1.0 MB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatBytes(tc.bytes)
			if result != tc.expected {
				t.Errorf("formatBytes(%d) = %s; want %s", tc.bytes, result, tc.expected)
			}
		})
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	sizes := []int64{
		512,                       // 512 B
		1024,                      // 1 KB
		1024 * 1024,               // 1 MB
		1024 * 1024 * 1024,        // 1 GB
		1024 * 1024 * 1024 * 1024, // 1 TB
	}

	for _, size := range sizes {
		b.Run(formatBytes(size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = formatBytes(size)
			}
		})
	}
}
