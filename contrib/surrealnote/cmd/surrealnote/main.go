package main

import (
	"context"
	"log"
	"os"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote"
)

func main() {
	// Execute the application with command line arguments
	// Use context.Background() for the main entry point
	if err := surrealnote.Main(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
