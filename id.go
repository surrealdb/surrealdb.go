package surrealdb

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var randSource = rand.New(rand.NewSource(time.Now().UnixNano()))
var randSourceLock sync.Mutex

func xid(length int) string {
	// Create a random byte slice
	b := make([]byte, length)
	// Fill the byte slice with data
	randSourceLock.Lock()
	randSource.Read(b)
	randSourceLock.Unlock()
	// Return the byte slice as a string
	return fmt.Sprintf("%x", b)[:length]
}
