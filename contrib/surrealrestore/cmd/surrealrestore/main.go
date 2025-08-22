package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/surrealdb/surrealdb.go/contrib/surrealrestore"
)

func main() {
	// Create config with defaults
	config := surrealrestore.NewConfig()

	// Parse command-line flags into config
	flag.StringVar(&config.Endpoint, "endpoint", config.Endpoint, "SurrealDB server endpoint")
	flag.StringVar(&config.Username, "username", config.Username, "Authentication username")
	flag.StringVar(&config.Password, "password", config.Password, "Authentication password")
	flag.StringVar(&config.Input, "input", "", "Input file path for single dump restore (use -dir for chain restoration)")
	flag.BoolVar(&config.Incremental, "incremental", false, "Perform incremental restore (used with -input)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Uint64Var(&config.PointInTime, "point-in-time", 0, "Restore to specific versionstamp (used with -dir)")
	flag.StringVar(&config.Dir, "dir", "", "Directory containing dump chain (alternative to -input)")
	flag.BoolVar(&config.Latest, "latest", false, "Restore to latest available versionstamp (used with -dir)")
	flag.BoolVar(&config.Info, "info", false, "Show dump chain information only (used with -dir)")

	flag.Parse()

	// Validate configuration
	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Execute the restore
	ctx := context.Background()
	if err := surrealrestore.Do(ctx, config); err != nil {
		log.Fatal(err)
	}
}
