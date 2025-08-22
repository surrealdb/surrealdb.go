# surrealrestore

A Go-based tool for restoring SurrealDB databases from CBOR format dumps created by [surrealdump](../surrealdump).

> This tool is for demonstrating the Go SDK usage and not intended for production use

## Features

- Full database restore from CBOR dumps
- Incremental restore using change feed data
- Automatic namespace and database creation
- Preserves record IDs and relationships
- Detailed restore statistics
- Dump chain validation with manifest files
- Point-in-time recovery to specific versionstamps
- Automatic chain discovery and validation
- Safety guards against incompatible incremental dumps

## Installation

```bash
go install github.com/surrealdb/surrealdb.go/contrib/surrealrestore/cmd/surrealrestore@latest
```

## Usage

- [Full restore](#full-restore) creates namespaces and databases as needed
- [Incremental restore](#incremental-restore) applies changes in order based on versionstamps
- [Scan dump chain](#scan-dump-chains) to review restoration points
- [Point-in-time restore](#point-in-time-restore) to a specific restoration point
- [Starting another dump chain](#starting-another-dump-chain) after the restoration

### Full Restore

Restore a complete database from a full dump (requires clean or non-existent target database):

```bash
surrealrestore -endpoint ws://localhost:8000 -username root -password root -input dump.cbor
```

### Incremental Restore

Apply incremental changes from an incremental dump:

```bash
surrealrestore -incremental -endpoint ws://localhost:8000 -username root -password root -input increment.cbor
```

### Scan Dump Chains

View available dump chains and restore points:

```bash
# Show dump chain information
surrealrestore -dir backups/

# Or explicitly with -info flag
surrealrestore -dir backups/ -info
```

### Point-in-Time Restore

Restore a database to a specific point in time using a dump chain:

```bash
# Restore to specific versionstamp
surrealrestore -dir backups/ -point-in-time 655360 -endpoint ws://localhost:8000 -username root -password root

# Restore to latest available state
surrealrestore -dir backups/ -latest -endpoint ws://localhost:8000 -username root -password root
```

### Starting another dump chain

You can apply incremental dumps from the original database to a restored database, but you cannot continue the dump chain after restoration.

This is because restored databases have different versionstamps than the original database it was restored from.

To start a new backup chain:

1. **Re-enable Change Feeds** on the restored database:
   ```sql
   DEFINE DATABASE OVERWRITE <database_name> CHANGEFEED <retention_period>
   ```

   > This is necessary because change Feeds are not preserved during restoration.
   > You must manually re-enable Change Feeds on the restored database if you want to create incremental dumps from it.

2. **Take a new full dump** of the restored database to establish a new baseline:
   ```bash
   surrealdump -endpoint ws://localhost:8001 -namespace restored -database prod -output new-full.cbor
   ```

3. **Continue with incremental dumps** based on the new full dump:
   ```bash
   surrealdump -incremental -endpoint ws://localhost:8001 -namespace restored -database prod -output new-inc.cbor
   ```

### Options

- `-endpoint`: SurrealDB server endpoint (default: ws://localhost:8000)
- `-username`: Authentication username (default: root)
- `-password`: Authentication password (default: root)
- `-input`: Input dump file path (required for single file restore)
- `-incremental`: Perform incremental restore from change feed data
- `-verbose`: Enable verbose logging for debugging
- `-validate`: Validate dump chain using manifests (default: true)
- `-dir`: Directory containing dump chain (shows chain info when used alone, or with -info flag)
- `-info`: Show dump chain information without restoring (use with -dir)
- `-latest`: Restore to latest available versionstamp (use with -dir)
- `-point-in-time`: Restore to specific versionstamp (use with -dir)

## Example Usage

### Creating a Backup and Restore Strategy

```bash
# Enable Change Feeds first (required for incremental dumps)
echo "DEFINE DATABASE OVERWRITE prod CHANGEFEED 1h" | surreal sql -e ws://localhost:8000 -u root -p root --ns myapp

# Create initial full backup
surrealdump -endpoint ws://localhost:8000 -namespace myapp -database prod -dir backups -output full-001.cbor

# Create incremental backups (auto-detects base from directory)
surrealdump -incremental -namespace myapp -database prod -dir backups -output inc-002.cbor
surrealdump -incremental -namespace myapp -database prod -dir backups -output inc-003.cbor

# View the backup chain
surrealrestore -dir backups/

# Restore to a specific point
surrealrestore -dir backups/ -point-in-time 655360 -endpoint ws://localhost:8001

# Or restore to latest state
surrealrestore -dir backups/ -latest -endpoint ws://localhost:8001

# Re-enable Change Feeds on restored database for new backup chain
echo "DEFINE DATABASE OVERWRITE prod CHANGEFEED 1h" | surreal sql -e ws://localhost:8001 -u root -p root --ns myapp

# Start new backup chain from restored database
surrealdump -endpoint ws://localhost:8001 -namespace myapp -database prod -dir restored-backups -output full-001.cbor
```

### Manual Step-by-Step Restore

```bash
# First restore the full dump
surrealrestore -endpoint ws://localhost:8001 -input backups/full-001.cbor

# Then apply incremental changes in order
surrealrestore -incremental -endpoint ws://localhost:8001 -input backups/inc-002.cbor
surrealrestore -incremental -endpoint ws://localhost:8001 -input backups/inc-003.cbor
```

## Safety Guards

The restore tool includes automatic validation to prevent data corruption:
- Validates dump type matches restore mode (full vs incremental)
- Checks versionstamp continuity in dump chains
- Prevents applying incremental dumps with gaps
- Ensures namespace/database consistency across dumps

For detailed information about dump chain validation and safety mechanisms, see the [surrealdump design documentation](../surrealdump/docs/design.md).
