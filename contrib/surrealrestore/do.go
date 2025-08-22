package surrealrestore

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
)

// Do executes the restore operation based on the configuration.
// It performs either:
//   - Directory-based operation: Restores DB to a specified or latest versionstamp by
//     replaying full and incremental dumps in creation order
//   - File-based operation: Applies a specific dump file (full or incremental) to the
//     target DB. User must ensure correct order (full before incremental) to avoid
//     inconsistent state
func Do(ctx context.Context, config *Config) error {
	// Handle directory-based operations (point-in-time restoration from dump chain)
	if config.Dir != "" {
		return performDirectoryRestore(ctx, config)
	}

	// Handle file-based operations (single dump file restoration)
	return performFileRestore(ctx, config)
}

// performDirectoryRestore handles restoration from a directory containing a dump chain.
// It can restore to a specific versionstamp or the latest available versionstamp.
func performDirectoryRestore(ctx context.Context, config *Config) error {
	// If only showing info, display chain information
	if config.Info || (!config.Latest && config.PointInTime == 0) {
		return showChainInfo(config.Dir)
	}

	return performPointInTimeRestore(ctx, config)
}

// performFileRestore handles restoration from a single dump file.
// It applies either a full or incremental dump based on the file's manifest.
func performFileRestore(ctx context.Context, config *Config) error {
	// Create restorer
	restorer, cleanup, err := newRestorer(ctx, config)
	if err != nil {
		return err
	}
	defer cleanup()

	startTime := time.Now()

	if config.Incremental {
		log.Println("Starting incremental restore...")
		if err := restorer.Incremental(ctx, config.Input); err != nil {
			return fmt.Errorf("incremental restore failed: %w", err)
		}
		log.Printf("Incremental restore completed successfully")
	} else {
		log.Println("Starting full database restore...")
		if err := restorer.Full(ctx, config.Input); err != nil {
			return fmt.Errorf("full restore failed: %w", err)
		}
		log.Printf("Full restore completed successfully")
	}

	elapsed := time.Since(startTime)
	stats := restorer.Stats()

	// Display statistics
	log.Printf("Restore completed in %v", elapsed)
	if !config.Incremental {
		log.Printf("Statistics:")
		log.Printf("  - Records restored: %d", stats.RecordsRestored)
		log.Printf("  - Tables restored: %d", stats.TablesRestored)
		log.Printf("  - Namespaces created: %d", stats.NamespacesCreated)
		log.Printf("  - Databases created: %d", stats.DatabasesCreated)
	} else {
		log.Printf("  - Changes applied: %d", stats.ChangesApplied)
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

func showChainInfo(dir string) error {
	// Scan, build and validate chains
	chains, err := surrealdump.ScanChains(dir)
	if err != nil {
		return err
	}

	if len(chains) == 0 {
		fmt.Println("No valid dump chains found in directory")
		return nil
	}

	fmt.Printf("Found %d valid dump chain(s) in %s:\n\n", len(chains), dir)

	for i, chain := range chains {
		fmt.Printf("Chain %d: %s.%s\n", i+1, chain.FullDump.Namespace, chain.FullDump.Database)
		fmt.Println("  Dumps:")
		fmt.Printf("    [FULL] %s (vs: %d)\n", chain.FullDump.Filename, chain.FullDump.EndVersionstamp)

		for _, inc := range chain.IncrementalDumps {
			fmt.Printf("    [INCR] %s (vs: %d -> %d)\n",
				inc.Filename, inc.StartVersionstamp, inc.EndVersionstamp)
		}

		fmt.Printf("\n  Available restore points:\n")
		points := chain.GetRestorationPoints()
		for j, vs := range points {
			marker := ""
			if j == len(points)-1 {
				marker = " (latest)"
			}
			fmt.Printf("    - Versionstamp %d%s\n", vs, marker)
		}

		fmt.Printf("\n  Total size: %s\n", formatBytes(chain.TotalSize))
		fmt.Printf("  ✓ Chain validated successfully\n")

		fmt.Println()
	}

	fmt.Println("To restore, use one of:")
	fmt.Printf("  surrealrestore -dir %s -latest\n", dir)
	fmt.Printf("  surrealrestore -dir %s -point-in-time <versionstamp>\n", dir)

	return nil
}

func performPointInTimeRestore(ctx context.Context, config *Config) error {
	// Scan, build, and validate chains
	chains, err := surrealdump.ScanChains(config.Dir)
	if err != nil {
		return err
	}

	if len(chains) == 0 {
		return fmt.Errorf("no valid dump chains found in directory")
	}

	// Use the first valid chain (could be enhanced to let user choose)
	chain := chains[0]

	// Determine target versionstamp
	targetVs := config.PointInTime
	if targetVs == 0 {
		targetVs = chain.LatestVersionstamp
		log.Printf("Restoring to latest versionstamp: %d", targetVs)
	} else {
		log.Printf("Restoring to versionstamp: %d", targetVs)
	}

	// Get dumps needed for restoration
	dumps, err := chain.GetManifestsForVersionstamp(targetVs)
	if err != nil {
		return fmt.Errorf("failed to get dumps for versionstamp %d: %w", targetVs, err)
	}

	log.Printf("Restoring to versionstamp %d using %d dump(s)", targetVs, len(dumps))

	// Create restorer
	restorer, cleanup, err := newRestorer(ctx, config)
	if err != nil {
		return err
	}
	defer cleanup()

	// Apply dumps in order
	for i, manifest := range dumps {
		dumpPath := filepath.Join(config.Dir, manifest.Filename)
		log.Printf("Applying dump %d/%d: %s", i+1, len(dumps), manifest.Filename)

		file, err := os.Open(dumpPath)
		if err != nil {
			return fmt.Errorf("failed to open dump file %s: %w", dumpPath, err)
		}

		if i == 0 {
			// First dump is always full
			if err := restorer.fullFromManifest(ctx, dumpPath, manifest); err != nil {
				file.Close()
				return fmt.Errorf("failed to restore full dump: %w", err)
			}
		} else {
			// Subsequent dumps are incremental
			if err := restorer.incrementalFromManifest(ctx, dumpPath, manifest); err != nil {
				file.Close()
				return fmt.Errorf("failed to apply incremental dump: %w", err)
			}
		}

		file.Close()
	}

	stats := restorer.Stats()
	log.Printf("Point-in-time restore completed to versionstamp %d", targetVs)
	log.Printf("  Records restored: %d", stats.RecordsRestored)
	log.Printf("  Changes applied:  %d", stats.ChangesApplied)

	return nil
}
