package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
)

func main() {
	// Create config with defaults
	config := surrealdump.NewConfig()

	// Parse command-line flags into config
	flag.StringVar(&config.Endpoint, "endpoint", config.Endpoint, "SurrealDB server endpoint")
	flag.StringVar(&config.Username, "username", config.Username, "Authentication username")
	flag.StringVar(&config.Password, "password", config.Password, "Authentication password")
	flag.StringVar(&config.Namespace, "namespace", "", "Namespace to dump (required)")
	flag.StringVar(&config.Database, "database", "", "Database to dump (required)")
	flag.StringVar(&config.Output, "output", "", "Output file path (required)")
	flag.BoolVar(&config.Incremental, "incremental", false, "Perform incremental dump")
	flag.Uint64Var(&config.SinceVersionstamp, "since", 0, "Versionstamp to start incremental dump from")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&config.Dir, "dir", "", "Base directory for dumps (prefixes output path)")

	// Tables flag - comma-separated list of tables to dump
	var tablesFlag string
	flag.StringVar(&tablesFlag, "tables", "", "Comma-separated list of tables to dump (empty means all tables)")

	flag.Parse()

	// Parse tables flag
	if tablesFlag != "" {
		config.Tables = strings.Split(tablesFlag, ",")
		// Trim spaces from table names
		for i, table := range config.Tables {
			config.Tables[i] = strings.TrimSpace(table)
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Execute the dump
	ctx := context.Background()
	if err := surrealdump.Do(ctx, config); err != nil {
		log.Fatal(err)
	}
}
