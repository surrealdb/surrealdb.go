package surrealdump

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// Alias types from surrealcbor for convenience
type (
	Encoder = surrealcbor.Encoder
	Decoder = surrealcbor.Decoder
)

// Helper functions to create encoder/decoder
var (
	NewEncoder = surrealcbor.NewEncoder
	NewDecoder = surrealcbor.NewDecoder
)

// DumpFormat represents the format version of the dump
const DumpFormat = "SURDUMP01"

// Record represents a single database record
type Record struct {
	Table string         `cbor:"table"`
	ID    string         `cbor:"id"`
	Data  map[string]any `cbor:"data"`
}

// ChangeEntry represents a change feed entry
type ChangeEntry struct {
	Table        string   `cbor:"table"`
	Versionstamp uint64   `cbor:"versionstamp"`
	Changes      []Change `cbor:"changes"`
}

// Change represents a single change in the database
type Change struct {
	DefineTable *ChangeDefineTable `cbor:"define_table,omitempty"`
	Update      map[string]any     `cbor:"update,omitempty"`
	Delete      map[string]any     `cbor:"delete,omitempty"`
}

// ChangeDefineTable represents the definition of a new table
type ChangeDefineTable struct {
	Name string `cbor:"name"`
}

// Dumper handles the database dump process.
// It dumps the currently selected namespace and database that was set via db.Use().
// To dump a specific namespace/database, call db.Use(ctx, namespace, database) before creating the dumper.
type Dumper struct {
	db        *surrealdb.DB
	codec     *surrealcbor.Codec
	namespace string
	database  string
	tables    []string // Tables to dump, if empty will dump all tables
}

// New creates a new Dumper instance.
//
// The provided db connection MUST have the namespace and database already selected
// via db.Use(ctx, namespace, database) BEFORE creating the Dumper. The namespace and database
// parameters passed here are for metadata purposes only - they do NOT change the connection's
// current namespace/database context.
//
// The namespace and database parameters are required because SurrealDB doesn't provide an API
// to query the current context, so we need them for writing metadata.
//
// Example:
//
//	db.Use(ctx, "myapp", "production")  // Set namespace/database first
//	dumper := surrealdump.New(db, "myapp", "production")  // Pass same values for metadata
func New(db *surrealdb.DB, namespace, database string, tables ...string) *Dumper {
	return &Dumper{
		db:        db,
		codec:     surrealcbor.New(),
		namespace: namespace,
		database:  database,
		tables:    tables,
	}
}

// full performs a consistent full database dump to a writer.
// This is a private method - external callers should use Full() which writes to a file
// and creates the mandatory manifest.
//
//nolint:gocyclo // Complex logic required for consistent dump algorithm
func (d *Dumper) full(ctx context.Context, w io.Writer) (*uint64, error) {
	if _, err := w.Write([]byte(DumpFormat)); err != nil {
		return nil, fmt.Errorf("failed to write magic header: %w", err)
	}

	// Use configured tables or detect all tables if not set
	tables := d.tables
	if len(tables) == 0 {
		detectedTables, err := detectTables(ctx, d.db)
		if err != nil {
			return nil, fmt.Errorf("failed to detect tables in current database: %w", err)
		}
		tables = detectedTables
	}

	// Change feeds are MANDATORY for consistent dumps
	for _, table := range tables {
		if feedErr := d.ensureChangeFeed(ctx, table); feedErr != nil {
			return nil, fmt.Errorf("failed to ensure change feed on table %s: %w", table, feedErr)
		}
	}

	// Get vs_0: the versionstamp BEFORE we start the dump
	// This ensures we capture all changes, even those made during the dump
	vs0, err := d.GetCurrentVersionstamp(ctx)
	if err != nil {
		// Log the error for debugging
		return nil, fmt.Errorf("failed to get initial versionstamp: %w", err)
	}

	encoder := NewEncoder(w)

	// PART 1: Dump all table records (inconsistent full dump between vs_1 and vs_2)
	for _, table := range tables {
		// Dump table - namespace/database are in metadata
		if dumpErr := d.dumpTable(ctx, encoder, table); dumpErr != nil {
			return nil, fmt.Errorf("failed to dump table %s: %w", table, dumpErr)
		}
	}

	// Get vs_2: versionstamp after dumping all tables
	vs2, err := d.GetCurrentVersionstamp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get end versionstamp: %w", err)
	}

	// PART 2: Capture ALL changes from vs_0 to vs_2
	// This ensures consistency by replaying any changes that occurred
	// before or during the dump
	for _, table := range tables {
		maxVs, err := d.getAndEncodeTableChanges(ctx, encoder, table, vs0)
		if err != nil {
			return nil, fmt.Errorf("failed to capture changes for table %s: %w", table, err)
		}

		// Track the actual end versionstamp we captured
		if maxVs > vs2 {
			vs2 = maxVs
		}
	}

	return &vs2, nil
}

func (d *Dumper) getAndEncodeTableChanges(
	ctx context.Context,
	encoder *surrealcbor.Encoder,
	table string,
	sinceVersionstamp uint64,
) (uint64, error) {
	changes, maxVs, err := d.getTableChanges(ctx, table, sinceVersionstamp)
	if err != nil {
		return 0, err
	}

	for _, changeSet := range changes {
		entry := ChangeEntry{
			Table:        table,
			Versionstamp: changeSet.Versionstamp,
			Changes:      changeSet.Changes,
		}
		if err := encoder.Encode(entry); err != nil {
			return maxVs, fmt.Errorf("failed to encode change entry: %w", err)
		}
	}

	return maxVs, nil
}

// incremental performs an incremental dump of changes to a writer.
// External callers should use Incremental() which writes to a file
// and creates the mandatory manifest.
func (d *Dumper) incremental(ctx context.Context, w io.Writer, sinceVersionstamp uint64) (*uint64, error) {
	if _, err := w.Write([]byte("SURINC01")); err != nil {
		return nil, fmt.Errorf("failed to write magic header: %w", err)
	}

	encoder := NewEncoder(w)

	var lastVersionstamp = sinceVersionstamp

	// Use configured tables or detect all tables if not set
	tables := d.tables
	if len(tables) == 0 {
		detectedTables, err := detectTables(ctx, d.db)
		if err != nil {
			return nil, fmt.Errorf("failed to detect tables: %w", err)
		}
		tables = detectedTables
	}

	// Collect changes from all tables in the current database
	for _, table := range tables {
		maxVs, err := d.getAndEncodeTableChanges(ctx, encoder, table, sinceVersionstamp)
		if err != nil {
			return nil, fmt.Errorf("failed to capture changes for table %s: %w", table, err)
		}

		if maxVs > lastVersionstamp {
			lastVersionstamp = maxVs
		}
	}

	// If no changes were found, return an error
	// An incremental dump without changes is meaningless
	if lastVersionstamp == sinceVersionstamp {
		return nil, fmt.Errorf("no changes captured since versionstamp %d - incremental dump would be empty", sinceVersionstamp)
	}

	return &lastVersionstamp, nil
}

// ensureChangeFeed ensures a table has change feed enabled
func (d *Dumper) ensureChangeFeed(ctx context.Context, table string) error {
	// OVERWRITE will update existing table or create new one
	query := fmt.Sprintf("DEFINE TABLE OVERWRITE %s CHANGEFEED 1h", table)
	_, err := surrealdb.Query[any](ctx, d.db, query, nil)
	return err
}

// detectTables detects all tables in the current database
func detectTables(ctx context.Context, db *surrealdb.DB) ([]string, error) {
	query := "INFO FOR DB"
	result, err := surrealdb.Query[map[string]any](ctx, db, query, nil)
	if err != nil {
		return nil, err
	}

	var tables []string
	if len(*result) > 0 {
		if info, ok := (*result)[0].Result["tables"].(map[string]any); ok {
			for table := range info {
				tables = append(tables, table)
			}
		}
	}
	return tables, nil
}

func (d *Dumper) dumpTable(ctx context.Context, encoder *Encoder, table string) error {
	// TODO: Enhancement for large tables - parallel/chunked SELECT
	//
	// For very large tables, we could potentially split the SELECT into parallel chunks
	// by ID ranges to improve dump performance:
	//
	// 1. Get min ID: SELECT VALUE id FROM table ORDER BY id ASC LIMIT 1
	// 2. Get max ID: SELECT VALUE id FROM table ORDER BY id DESC LIMIT 1
	// 3. Split ID range into N chunks
	// 4. Parallel SELECT for each chunk: SELECT * FROM table WHERE id >= $start AND id < $end
	//
	// However, this approach is currently blocked because:
	// - Not all SurrealDB datastores support reverse scans (ORDER BY id DESC)
	// - Without max ID, we can't determine the range to split
	// - Example error: "The underlying datastore does not support reversed scans"
	//
	// Alternative approaches that might work:
	// 1. Sequential pagination using last seen ID (memory efficient, works today):
	//    - Get first batch: SELECT * FROM table ORDER BY id ASC LIMIT 1000
	//    - Remember last ID from batch
	//    - Get next batch: SELECT * FROM table WHERE id > $lastSeenId ORDER BY id ASC LIMIT 1000
	//    - Repeat until no more records
	//    - Benefits: Low memory usage on client/server, works on all backends
	//    - Drawback: Sequential only, can't parallelize
	//
	// 2. Use COUNT() to estimate chunks (but still need max ID for ranges)
	// 3. Stream records with pagination using LIMIT/START (sequential, not parallel)
	// 4. Wait for SurrealDB to add reverse scan support to all datastores
	//
	// For now, we use a simple full table SELECT which works reliably on all backends.

	// Use surrealql to build the SELECT query
	q := surrealql.Select(table)
	query, _ := q.Build()

	result, err := surrealdb.Query[[]map[string]any](ctx, d.db, query, nil)
	if err != nil {
		return err
	}

	if len(*result) > 0 {
		for _, data := range (*result)[0].Result {
			record := Record{
				Table: table,
				Data:  data,
			}
			if id, ok := data["id"]; ok {
				record.ID = fmt.Sprintf("%v", id)
			}
			if err := encoder.Encode(record); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Dumper) getTableChanges(ctx context.Context, table string, sinceVersionstamp uint64) (allChanges []struct {
	Versionstamp uint64
	Changes      []Change
}, maxVs uint64, err error) {
	q := surrealql.ShowChangesForTable(table).SinceVersionstamp(sinceVersionstamp)
	query, _ := q.Build()

	type ChangeSet struct {
		Versionstamp uint64   `json:"versionstamp"`
		Changes      []Change `json:"changes"`
	}

	result, err := surrealdb.Query[[]ChangeSet](ctx, d.db, query, nil)
	if err != nil {
		return nil, 0, err
	}

	var maxVersionstamp = sinceVersionstamp

	if len(*result) > 0 {
		for _, changeSet := range (*result)[0].Result {
			allChanges = append(allChanges, struct {
				Versionstamp uint64
				Changes      []Change
			}{
				Versionstamp: changeSet.Versionstamp,
				Changes:      changeSet.Changes,
			})
			if changeSet.Versionstamp > maxVersionstamp {
				maxVersionstamp = changeSet.Versionstamp
			}
		}
	}

	return allChanges, maxVersionstamp, nil
}

// WriteUint64 writes a uint64 to the writer in big-endian format
func WriteUint64(w io.Writer, v uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	_, err := w.Write(buf)
	return err
}

// ReadUint64 reads a uint64 from the reader in big-endian format
func ReadUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(buf), nil
}

// Full performs a consistent full database dump to a file with mandatory manifest.
//
// The dump consists of two parts to ensure consistency:
// 1. An inconsistent full dump of all records (captured between vs_1 and vs_2)
// 2. All changes from vs_0 (before the dump started) to vs_2 (after the dump ended)
//
// By including the complete change history, we ensure that even if the inconsistent
// dump captured records at different points between vs_1 and vs_2, we can replay all
// changes up to vs_2 to get a consistent state. This approach guarantees that incremental
// dumps starting at vs_2 won't miss any changes.
//
// Example usage:
//
//	db.Use(ctx, "myapp", "production")
//	dumper := surrealdump.New(db, "myapp", "production")
//	dumper.Full(ctx, "/path/to/dump.cbor")
func (d *Dumper) Full(ctx context.Context, filePath string) error {
	startTime := time.Now()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	writer := io.MultiWriter(file, hash)

	// Perform the dump
	maxVs, dumpErr := d.full(ctx, writer)
	if dumpErr != nil {
		return fmt.Errorf("dump failed: %w", dumpErr)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	manifest := &Manifest{
		Filename:          filepath.Base(filePath),
		Type:              ManifestTypeFull,
		CreatedAt:         startTime,
		Size:              fileInfo.Size(),
		Namespace:         d.namespace,
		Database:          d.database,
		EndVersionstamp:   *maxVs,
		StartVersionstamp: 0, // Full dumps start from 0
		SHA256:            fmt.Sprintf("%x", hash.Sum(nil)),
	}

	if err := WriteManifest(filePath, manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// Incremental performs an incremental dump of changes since the specified versionstamp
// to a file with mandatory manifest.
//
// Returns an error if no changes have been captured since the given versionstamp,
// as empty incremental dumps are not valid.
//
// Example usage:
//
//	db.Use(ctx, "myapp", "production")
//	dumper := surrealdump.New(db, "myapp", "production")
//	dumper.Incremental(ctx, "/path/to/incremental.cbor", sinceVersionstamp)
func (d *Dumper) Incremental(ctx context.Context, filePath string, sinceVersionstamp uint64) error {
	startTime := time.Now()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	writer := io.MultiWriter(file, hash)

	// Perform the dump
	maxVs, dumpErr := d.incremental(ctx, writer, sinceVersionstamp)
	if dumpErr != nil {
		return fmt.Errorf("dump failed: %w", dumpErr)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	manifest := &Manifest{
		Filename:          filepath.Base(filePath),
		Type:              ManifestTypeIncremental,
		CreatedAt:         startTime,
		Size:              fileInfo.Size(),
		Namespace:         d.namespace,
		Database:          d.database,
		EndVersionstamp:   *maxVs,
		StartVersionstamp: sinceVersionstamp,
		SHA256:            fmt.Sprintf("%x", hash.Sum(nil)),
	}

	if err := WriteManifest(filePath, manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
