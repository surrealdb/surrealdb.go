package surrealdb

import (
	"fmt"
	"math/rand"
	"time"
)

func xid(length int) string {
	// Generate a new seed
	rand.Seed(time.Now().UnixNano())
	// Create a random byte slice
	b := make([]byte, length)
	// Fill the byte slice with data
	rand.Read(b)
	// Return the byte slice as a string
	return fmt.Sprintf("%x", b)[:length]
}
