package rand

import (
	"encoding/base64"
	"testing"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

func BenchmarkNewRequestID1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newRequestID1(constants.RequestIDLength)
	}
}

func BenchmarkNewRequestID2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newRequestID2(constants.RequestIDLength)
	}
}

func BenchmarkNewRequestID3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRequestID(constants.RequestIDLength)
	}
}

func BenchmarkNewRequestID4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newRequestID4(constants.RequestIDLength)
	}
}

func BenchmarkNewRequestID5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newRequestID5(constants.RequestIDLength)
	}
}

// Uniform distribution, fixed key length, much slower
func newRequestID5(requestIDLength int) string {
	buf := make([]byte, requestIDLength)
	defaultRandBytes.read(buf)

	index := 0
	for _, b := range buf {
		if b > unbiasedMaxVal {
			continue
		}
		buf[index] = charset[int(b)%charsetLen]
		index++
	}

	if index == requestIDLength {
		return string(buf)
	}

	bytes := make([]byte, bytesInUint64)

	for index < requestIDLength {
		defaultRandBytes.read(bytes)
		for _, b := range bytes {
			if b > unbiasedMaxVal {
				continue
			}
			buf[index] = charset[int(b)%charsetLen]
			index++
			if index == requestIDLength {
				break
			}
		}
	}

	return string(buf)
}

// Uniform distribution, but slower and variable key length (<= requestKeyLength).
func newRequestID4(requestIDLength int) string {
	buf := make([]byte, requestIDLength)
	defaultRandBytes.read(buf)

	index := 0
	for _, b := range buf {
		if b > unbiasedMaxVal {
			continue
		}
		buf[index] = charset[int(b)%charsetLen]
		index++
	}

	return string(buf[:index])
}

// newRequestID3 is NewRequestID from rand.go

// Using simple rng and base64.
func newRequestID2(requestIDLength int) string {
	buf := make([]byte, requestIDLength)
	defaultRandBytes.read(buf)

	return base64.RawURLEncoding.EncodeToString(buf)
}

// Similar to original method.
func newRequestID1(requestIDLength int) string {
	return defaultRandBytes.base62Str(requestIDLength)
}
