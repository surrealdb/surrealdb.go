package surrealdb

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
var randLock sync.Mutex

func xid(length int) string {
	// Generate a new seed
	// Create a random byte slice
	b := make([]byte, length)
	// Fill the byte slice with data
	randLock.Lock()
	randSource.Read(b) //nolint:gosec
	randLock.Unlock()
	// Return the byte slice as a string
	return fmt.Sprintf("%x", b)[:length]
}
