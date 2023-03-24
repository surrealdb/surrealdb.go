package rand

import (
	"math/rand"
	"sync"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New( //nolint:gosec
	rand.NewSource(time.Now().UnixNano()))
var seededRandLock sync.Mutex

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	seededRandLock.Lock()
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	seededRandLock.Unlock()
	return string(b)
}

func String(length int) string {
	return StringWithCharset(length, charset)
}
