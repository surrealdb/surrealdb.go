package surrealdump

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// newDumper creates a new dumper from the configuration
func newDumper(ctx context.Context, config *Config) (*Dumper, func(), error) {
	u, err := url.ParseRequestURI(config.Endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse server endpoint: %w", err)
	}

	conf := connection.NewConfig(u)
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec

	if !config.Verbose {
		conf.Logger = nil
	}

	conn := gws.New(conf)

	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SurrealDB: %w", err)
	}

	cleanup := func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			log.Printf("Warning: failed to close database connection: %v", closeErr)
		}
	}

	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: config.Username,
		Password: config.Password,
	})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	if err := db.Use(ctx, config.Namespace, config.Database); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to use namespace/database: %w", err)
	}

	dumper := New(db, config.Namespace, config.Database, config.Tables...)

	return dumper, cleanup, nil
}

// Do executes a dump operation based on the provided configuration.
// It handles connection setup, authentication, and performs either a full or incremental dump.
// The configuration should be validated before calling this function.
func Do(ctx context.Context, config *Config) error {
	outputPath := config.GetOutputPath()

	dumper, cleanup, err := newDumper(ctx, config)
	if err != nil {
		return err
	}
	defer cleanup()

	startTime := time.Now()

	if config.Incremental {
		// Auto-detect start versionstamp if not provided
		if config.SinceVersionstamp == 0 {
			if vs, err := config.findLatestVersionstamp(); err == nil && vs > 0 {
				config.SinceVersionstamp = vs
				log.Printf("Auto-detected start versionstamp: %d", vs)
			}
		}

		if config.SinceVersionstamp == 0 {
			return fmt.Errorf("no start versionstamp specified and no previous dumps found")
		}

		log.Printf("Starting incremental dump from versionstamp %d...", config.SinceVersionstamp)
		if err := dumper.Incremental(ctx, outputPath, config.SinceVersionstamp); err != nil {
			return fmt.Errorf("incremental dump failed: %w", err)
		}
		log.Printf("Incremental dump completed successfully")
	} else {
		log.Println("Starting full database dump...")
		if err := dumper.Full(ctx, outputPath); err != nil {
			return fmt.Errorf("full dump failed: %w", err)
		}
		log.Printf("Full dump completed successfully")
	}

	elapsed := time.Since(startTime)
	fileInfo, _ := os.Stat(outputPath)
	log.Printf("Dump completed in %v, output size: %s", elapsed, formatBytes(fileInfo.Size()))

	if !config.Incremental {
		if manifest, err := ReadManifest(outputPath); err == nil {
			displayDumpManifest(manifest)
		}
	}

	if config.Verbose {
		if manifest, err := ReadManifest(outputPath); err == nil {
			log.Printf("Manifest created: %s.manifest.json (SHA256: %s)", outputPath, manifest.SHA256)
		}
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func displayDumpManifest(manifest *Manifest) {
	fmt.Println("\nDump Information:")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Type:              %s\n", manifest.Type)
	fmt.Printf("Created At:        %s\n", manifest.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Namespace:         %s\n", manifest.Namespace)
	fmt.Printf("Database:          %s\n", manifest.Database)
	if manifest.Type == ManifestTypeFull {
		fmt.Printf("Versionstamp:      %d (full dump from 0)\n", manifest.EndVersionstamp)
	} else {
		fmt.Printf("Versionstamp Range: %d - %d\n", manifest.StartVersionstamp, manifest.EndVersionstamp)
	}
	if manifest.Type == ManifestTypeIncremental {
		fmt.Printf("Start Versionstamp:  %d\n", manifest.StartVersionstamp)
	}
	fmt.Printf("Size:              %s\n", formatBytes(manifest.Size))
	fmt.Printf("SHA256:            %s\n", manifest.SHA256)
}
