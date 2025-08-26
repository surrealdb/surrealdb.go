package surrealrestore

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// Alias types from surrealcbor for convenience
type (
	Decoder = surrealcbor.Decoder
)

// Helper functions to create decoder
var (
	NewDecoder = surrealcbor.NewDecoder
)

// RestoreStats tracks restoration statistics
type RestoreStats struct {
	RecordsRestored   int
	TablesRestored    int
	NamespacesCreated int
	DatabasesCreated  int
	ChangesApplied    int
	StartTime         time.Time
	EndTime           time.Time
}

// Restorer handles the database restore process
type Restorer struct {
	db    *surrealdb.DB
	codec *surrealcbor.Codec
	stats RestoreStats

	Verbose bool
	// Target namespace for restoration
	Namespace string
	// Target database for restoration
	Database string
}

// New creates a new Restorer instance
func New(db *surrealdb.DB) *Restorer {
	return &Restorer{
		db:    db,
		codec: surrealcbor.New(),
		stats: RestoreStats{
			StartTime: time.Now(),
		},
	}
}

// SetNamespace sets the target namespace for restoration
func (r *Restorer) SetNamespace(ns string) {
	r.Namespace = ns
}

// SetDatabase sets the target database for restoration
func (r *Restorer) SetDatabase(db string) {
	r.Database = db
}

// SetTarget sets both namespace and database for restoration
func (r *Restorer) SetTarget(ns, db string) {
	r.Namespace = ns
	r.Database = db
}

// Stats returns the restoration statistics
func (r *Restorer) Stats() RestoreStats {
	return r.stats
}

// fullFromReader performs a full database restore from the reader
//
// The dump format no longer includes metadata - it only contains:
// 1. Magic header
// 2. Table records (inconsistent full dump)
// 3. Change entries (to ensure consistency)
//
// Namespace and database should be set via SetTarget() or use FullFromManifest()
//
//nolint:gocyclo,funlen // Complex restore logic for handling multiple data formats
func (r *Restorer) fullFromReader(ctx context.Context, currentNamespace, currentDatabase string, reader io.Reader) error {
	r.stats.StartTime = time.Now()

	// Read magic header
	magic := make([]byte, len(surrealdump.DumpFormat))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return fmt.Errorf("failed to read magic header: %w", err)
	}

	if string(magic) != surrealdump.DumpFormat {
		return fmt.Errorf("invalid dump format: expected %s, got %s", surrealdump.DumpFormat, string(magic))
	}

	decoder := NewDecoder(reader)

	// Track which namespaces/databases we've created
	createdNS := make(map[string]bool)
	createdDB := make(map[string]map[string]bool)
	restoredTables := make(map[string]bool)

	// Buffer changes to apply them after all records
	var bufferedChanges []struct {
		Table  string
		Change surrealdump.Change
	}

	for {
		var item any
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode item: %w", err)
		}

		// Check if this is a map that could be a Record or ChangeEntry
		if itemMap, ok := item.(map[string]any); ok {
			// Check if it's a ChangeEntry (has versionstamp and changes)
			if _, hasVersionstamp := itemMap["versionstamp"]; hasVersionstamp {
				if _, hasChanges := itemMap["changes"]; hasChanges {
					// This is a change entry - buffer it to apply after all records
					if currentNamespace != "" && currentDatabase != "" {
						entryBytes, err := r.codec.Marshal(item)
						if err == nil {
							var entry surrealdump.ChangeEntry
							if err := r.codec.Unmarshal(entryBytes, &entry); err == nil {
								// Buffer the changes for later
								for _, change := range entry.Changes {
									bufferedChanges = append(bufferedChanges, struct {
										Table  string
										Change surrealdump.Change
									}{
										Table:  entry.Table,
										Change: change,
									})
								}
							}
						}
					}
					continue
				}
			}

			// Check if it has the structure of a Record
			if _, hasTable := itemMap["table"]; hasTable {
				if _, hasData := itemMap["data"]; hasData {
					// This looks like a Record
					recordBytes, err := r.codec.Marshal(item)
					if err != nil {
						if r.Verbose {
							log.Printf("Failed to marshal record: %v", err)
						}
						continue
					}

					var record surrealdump.Record
					if err := r.codec.Unmarshal(recordBytes, &record); err != nil {
						if r.Verbose {
							log.Printf("Failed to unmarshal record: %v", err)
						}
						continue
					}

					// Ensure we have namespace and database set
					if currentNamespace == "" || currentDatabase == "" {
						if r.Verbose {
							log.Printf("Warning: No namespace/database set, skipping record")
						}
						continue
					}

					// Ensure namespace and database exist
					r.ensureNamespaceDatabase(ctx, currentNamespace, currentDatabase, createdNS, createdDB)

					// Switch to the correct namespace and database
					if err := r.db.Use(ctx, currentNamespace, currentDatabase); err != nil {
						return fmt.Errorf("failed to use ns/db: %w", err)
					}

					// Track restored tables
					tableKey := fmt.Sprintf("%s.%s.%s", currentNamespace, currentDatabase, record.Table)
					if !restoredTables[tableKey] {
						restoredTables[tableKey] = true
						r.stats.TablesRestored++

						if r.Verbose {
							log.Printf("Restoring table: %s", tableKey)
						}
					}

					// Restore the record
					if record.Data != nil {
						// Always use UPSERT for records from the inconsistent dump
						// because they might have been partially updated by changes
						recordID := ""
						if idVal, ok := record.Data["id"]; ok {
							recordID = formatRecordID(idVal)
						}

						var query string
						if recordID != "" {
							// Use UPSERT when we have an ID to ensure we update if it exists
							delete(record.Data, "id") // Remove id from data
							query = fmt.Sprintf("UPSERT %s SET", recordID)
						} else {
							// Use CREATE for records without specific IDs
							q := surrealql.Create(record.Table)
							query, _ = q.Build()
						}

						// Convert map to SET clause (for both CREATE and UPSERT)
						var setItems []string
						vars := make(map[string]any)
						i := 0
						for k, v := range record.Data {
							paramName := fmt.Sprintf("param_%d", i)
							setItems = append(setItems, fmt.Sprintf("%s = $%s", k, paramName))
							vars[paramName] = v
							i++
						}

						if len(setItems) > 0 {
							query = fmt.Sprintf("%s %s", query, strings.Join(setItems, ", "))
						}

						_, err := surrealdb.Query[any](ctx, r.db, query, vars)
						if err != nil {
							// Try with INSERT if CREATE/UPSERT fails
							insertQuery := fmt.Sprintf("INSERT INTO %s $data", record.Table)
							_, err = surrealdb.Query[any](ctx, r.db, insertQuery, map[string]any{
								"data": record.Data,
							})
							if err != nil {
								return fmt.Errorf("failed to restore record: %w", err)
							}
						}

						r.stats.RecordsRestored++
					}
					continue
				}
			}

			// Note: Dumps no longer contain metadata - namespace/database must be set via SetTarget()
		}
	}

	// Now apply all buffered changes AFTER all records have been restored
	// This ensures changes overwrite the inconsistent dump data
	if currentNamespace != "" && currentDatabase != "" {
		if err := r.db.Use(ctx, currentNamespace, currentDatabase); err != nil {
			return fmt.Errorf("failed to use ns/db for changes: %w", err)
		}

		for i, bufferedChange := range bufferedChanges {
			if r.Verbose {
				if bufferedChange.Change.Update != nil {
					if id, ok := bufferedChange.Change.Update["id"]; ok {
						value := bufferedChange.Change.Update["value"]
						log.Printf("Change %d: Applying UPDATE to %v with value=%v", i+1, formatRecordID(id), value)
					}
				} else if bufferedChange.Change.Delete != nil {
					if id, ok := bufferedChange.Change.Delete["id"]; ok {
						log.Printf("Change %d: Applying DELETE to %v", i+1, formatRecordID(id))
					}
				} else if bufferedChange.Change.DefineTable != nil {
					log.Printf("Change %d: Defining table %s", i+1, bufferedChange.Change.DefineTable.Name)
				}
			}
			if err := r.applyChange(ctx, bufferedChange.Table, bufferedChange.Change); err != nil {
				if r.Verbose {
					log.Printf("Warning: failed to apply buffered change: %v", err)
				}
			} else {
				r.stats.ChangesApplied++
			}
		}
	}

	r.stats.EndTime = time.Now()
	return nil
}

// incrementalFromReader applies incremental changes from the reader
//
// Namespace and database should be set via SetTarget() or use IncrementalFromManifest()
//
//nolint:gocyclo // Complex logic for processing incremental changes
func (r *Restorer) incrementalFromReader(ctx context.Context, currentNamespace, currentDatabase string, reader io.Reader) error {
	// Read magic header
	magic := make([]byte, 8) // "SURINC01"
	if _, err := io.ReadFull(reader, magic); err != nil {
		return fmt.Errorf("failed to read magic header: %w", err)
	}

	if string(magic) != "SURINC01" {
		return fmt.Errorf("invalid incremental dump format: %s", string(magic))
	}

	decoder := NewDecoder(reader)

	for {
		var item any
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode item: %w", err)
		}

		// Try to decode as ChangeEntry
		entryBytes, err := r.codec.Marshal(item)
		if err != nil {
			continue // Might be metadata
		}

		var entry surrealdump.ChangeEntry
		if err := r.codec.Unmarshal(entryBytes, &entry); err == nil {
			// Use namespace and database from metadata
			if currentNamespace != "" && currentDatabase != "" {
				// Switch to the correct namespace and database
				if err := r.db.Use(ctx, currentNamespace, currentDatabase); err != nil {
					return fmt.Errorf("failed to use ns/db: %w", err)
				}

				// Apply each change
				for _, change := range entry.Changes {
					if err := r.applyChange(ctx, entry.Table, change); err != nil {
						if r.Verbose {
							log.Printf("Warning: failed to apply change: %v", err)
						}
						// Continue with other changes even if one fails
					} else {
						r.stats.ChangesApplied++
					}
				}
			}
		}

		// Check if this is metadata to extract namespace/database
		if itemMap, ok := item.(map[string]any); ok {
			if _, hasFormat := itemMap["format"]; hasFormat {
				if _, hasVersion := itemMap["version"]; hasVersion {
					// This is metadata - extract namespace and database
					if ns, ok := itemMap["namespace"].(string); ok {
						currentNamespace = ns
					}
					if db, ok := itemMap["database"].(string); ok {
						currentDatabase = db
					}
					if r.Verbose {
						log.Printf("Found incremental metadata: namespace=%s, database=%s", currentNamespace, currentDatabase)
					}
				}
			}
		}
	}

	return nil
}

func (r *Restorer) ensureNamespaceDatabase(ctx context.Context, ns, db string,
	createdNS map[string]bool, createdDB map[string]map[string]bool) {
	// Create namespace if not exists
	if !createdNS[ns] {
		query := fmt.Sprintf("DEFINE NAMESPACE IF NOT EXISTS %s", ns)
		if _, err := surrealdb.Query[any](ctx, r.db, query, nil); err != nil {
			// Namespace might already exist, which is fine
			if r.Verbose {
				log.Printf("Note: namespace %s might already exist", ns)
			}
		} else {
			r.stats.NamespacesCreated++
		}
		createdNS[ns] = true
		createdDB[ns] = make(map[string]bool)
	}

	// Create database if not exists
	if !createdDB[ns][db] {
		// First use the namespace
		if err := r.db.Use(ctx, ns, "temp"); err != nil {
			// Try without temp db
			_ = r.db.Use(ctx, ns, db)
		}

		query := fmt.Sprintf("DEFINE DATABASE IF NOT EXISTS %s", db)
		if _, err := surrealdb.Query[any](ctx, r.db, query, nil); err != nil {
			// Database might already exist, which is fine
			if r.Verbose {
				log.Printf("Note: database %s.%s might already exist", ns, db)
			}
		} else {
			r.stats.DatabasesCreated++
		}
		createdDB[ns][db] = true
	}
}

// formatRecordID converts a record ID from various formats to the correct string format
// e.g., from {products rwlrtb90sr1n5tslp5m8} to products:rwlrtb90sr1n5tslp5m8
func formatRecordID(id any) string {
	idStr := fmt.Sprintf("%v", id)
	// Check if it's in the format {table id}
	if strings.HasPrefix(idStr, "{") && strings.HasSuffix(idStr, "}") {
		// Remove braces and split by space
		idStr = strings.TrimPrefix(idStr, "{")
		idStr = strings.TrimSuffix(idStr, "}")
		parts := strings.Fields(idStr)
		if len(parts) == 2 {
			// Convert to table:id format
			return fmt.Sprintf("%s:%s", parts[0], parts[1])
		}
	}
	// If it's already in the correct format or unknown, return as is
	return idStr
}

func (r *Restorer) applyChange(ctx context.Context, _ string, change surrealdump.Change) error {
	if change.DefineTable != nil {
		// Define table
		query := fmt.Sprintf("DEFINE TABLE IF NOT EXISTS %s", change.DefineTable.Name)
		_, err := surrealdb.Query[any](ctx, r.db, query, nil)
		return err
	}

	if change.Update != nil {
		// Apply update (which could be a create or update)
		if id, ok := change.Update["id"]; ok {
			delete(change.Update, "id") // Remove id from data

			// Format the ID correctly (convert from map format to string)
			idStr := formatRecordID(id)

			// Always use UPSERT for changes to handle both create and update cases
			// This ensures we replace all fields with the change feed data
			query := fmt.Sprintf("UPSERT %s SET", idStr)

			// Convert map to SET clause
			var setItems []string
			vars := make(map[string]any)
			i := 0
			for k, v := range change.Update {
				paramName := fmt.Sprintf("upd_param_%d", i)
				setItems = append(setItems, fmt.Sprintf("%s = $%s", k, paramName))
				vars[paramName] = v
				i++
			}

			if len(setItems) > 0 {
				query = fmt.Sprintf("%s %s", query, strings.Join(setItems, ", "))
			}

			_, err := surrealdb.Query[any](ctx, r.db, query, vars)
			return err
		}
		return fmt.Errorf("update change missing id field")
	}

	if change.Delete != nil {
		// Apply delete
		if id, ok := change.Delete["id"]; ok {
			idStr := formatRecordID(id)
			q := surrealql.Delete(idStr)
			query, _ := q.Build()

			_, err := surrealdb.Query[any](ctx, r.db, query, nil)
			return err
		}
		return fmt.Errorf("delete change missing id field")
	}

	return nil
}

// Full performs a full database restore from a dump file path
// It automatically reads the manifest and restores the data to the appropriate namespace/database
func (r *Restorer) Full(ctx context.Context, dumpPath string) error {
	// Read the manifest
	manifest, err := surrealdump.ReadManifest(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	return r.fullFromManifest(ctx, dumpPath, manifest)
}

func (r *Restorer) fullFromManifest(ctx context.Context, dumpPath string, manifest *surrealdump.Manifest) error {
	// Verify it's a full dump
	if manifest.Type != surrealdump.ManifestTypeFull {
		return fmt.Errorf("expected full dump but got %s", manifest.Type)
	}

	ns, db, file, err := r.initRestore(dumpPath, manifest)
	if err != nil {
		return err
	}
	defer file.Close()

	return r.fullFromReader(ctx, ns, db, file)
}

// Incremental performs an incremental restore from a dump file path
// It automatically reads the manifest and applies the changes to the appropriate namespace/database
func (r *Restorer) Incremental(ctx context.Context, dumpPath string) error {
	// Read the manifest
	manifest, err := surrealdump.ReadManifest(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	return r.incrementalFromManifest(ctx, dumpPath, manifest)
}

func (r *Restorer) incrementalFromManifest(ctx context.Context, dumpPath string, manifest *surrealdump.Manifest) error {
	// Verify it's an incremental dump
	if manifest.Type != surrealdump.ManifestTypeIncremental {
		return fmt.Errorf("expected incremental dump but got %s", manifest.Type)
	}

	ns, db, file, err := r.initRestore(dumpPath, manifest)
	if err != nil {
		return err
	}
	defer file.Close()

	return r.incrementalFromReader(ctx, ns, db, file)
}

func (r *Restorer) initRestore(dumpPath string, manifest *surrealdump.Manifest) (ns, db string, dumpFile *os.File, err error) {
	ns = r.Namespace
	if ns == "" {
		ns = manifest.Namespace
	}

	db = r.Database
	if db == "" {
		db = manifest.Database
	}

	// Open the dump file
	file, err := os.Open(dumpPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to open dump file: %w", err)
	}

	return ns, db, file, nil
}
