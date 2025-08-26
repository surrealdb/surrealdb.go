# surrealdump

A Go-based tool for dumping SurrealDB databases to CBOR format, demonstrating the SurrealDB Go SDK capabilities with surrealcbor and gws.

> This tool demonstrates SDK capabilities and is not intended for production use.
>
> For production backups, use the official `surreal export` command.

## Features

- Consistent full dump combining inconsistent dump with Change Feed
- Incremental dumps using Change Feed
- CBOR-based binary format for efficient storage
- Metadata tracking (timestamps, versionstamps)
- Point-in-time recovery support
- SHA256 integrity verification

## Prerequisites for Incremental Dumps

Incremental dumps require Change Feeds to be enabled on the database **before** any data is created. Change Feeds must be configured using:

```sql
DEFINE DATABASE OVERWRITE <database_name> CHANGEFEED <retention_period>
```

For example:
```sql
DEFINE DATABASE OVERWRITE mydb CHANGEFEED 1h
```

Without Change Feeds enabled, full dumps will be inconsistent and incremental dumps will not capture any changes.

## Installation

```bash
go install github.com/surrealdb/surrealdb.go/contrib/surrealdump/cmd/surrealdump@latest
```

## Usage

`surrealdump` creates manifest files that track dump metadata and relationships, enabling safe point-in-time recovery and preventing incompatible incremental dumps from being applied.

```bash
# Create initial full backup
surrealdump -endpoint ws://localhost:8000 -namespace myapp -database prod -dir backups -output full-$(date +%Y%m%d).cbor

# Create daily incremental backups (auto-detects base from directory)
surrealdump -incremental -namespace myapp -database prod -dir backups -output inc-$(date +%Y%m%d-%H%M%S).cbor

# View backup chain
surrealrestore -dir backups/ -info

# Restore to specific versionstamp
surrealrestore -dir backups/ -point-in-time 655360 -endpoint ws://localhost:8001

# Restore to latest available state
surrealrestore -dir backups/ -endpoint ws://localhost:8001

# To start incremental backups on the restored database:
# First, take a new full dump
surrealdump -endpoint ws://localhost:8001 -namespace myapp -database prod -dir backups_new -output restored-full.cbor
# Now you can take incremental dumps based on the restored database
surrealdump -incremental -endpoint ws://localhost:8001 -namespace myapp -database prod -dir backups_new -output restored-inc1.cbor
```

Please refer to the following sections for more details on each feature:

- [Full Dump](#full-dump)
- [Incremental Dump](#incremental-dump)
- [Options](#options)
- [Restore with surrealrestore](#restore-with-surrealrestore)
- [Format](#format)
- [Notes](#notes)

### Full Dump

Full dumps include both an inconsistent full dump of records and change feed data to ensure consistency despite SurrealDB's isolation limitations. This approach guarantees no changes are missed when applying subsequent incremental dumps.

To take a full dump, run `surrealdump` without `-incremental`:

```bash
surrealdump -endpoint ws://localhost:8000 -namespace myapp -database production -username root -password root -output dump.cbor
```

### Incremental Dump

```bash
# Manual versionstamp specification
surrealdump -incremental -since 1000 -endpoint ws://localhost:8000 -namespace myapp -database production -username root -password root -output increment.cbor

# Auto-detect start versionstamp from directory
surrealdump -incremental -endpoint ws://localhost:8000 -namespace myapp -database production -username root -password root -dir backups/ -output increment.cbor
```

### Options

- `-endpoint`: SurrealDB server endpoint (default: ws://localhost:8000)
- `-username`: Authentication username (default: root)
- `-password`: Authentication password (default: root)
- `-namespace`: Namespace to dump (required)
- `-database`: Database to dump (required)
- `-output`: Output file path (required)
- `-incremental`: Perform incremental dump
- `-since`: Versionstamp to start incremental dump from (default: auto-detect from directory)
- `-verbose`: Enable verbose logging
- `-manifest`: Create manifest file for dump chain tracking (default: true)
- `-dir`: Base directory for dumps (prefixes the output path when used with -output)

### Restore with surrealrestore

To restore a dump created by surrealdump:

```bash
# Full restore (requires clean database)
surrealrestore -endpoint ws://localhost:8000 -username root -password root -input dump.cbor

# Apply incremental changes
surrealrestore -incremental -endpoint ws://localhost:8000 -username root -password root -input increment.cbor
```

See [surrealrestore documentation](../surrealrestore/) for more information.

### Format

Dumps use CBOR encoding with:
- Magic header: "SURDUMP01" (full) or "SURINC01" (incremental)
- Record entries containing table info and data
- Manifest files (`.manifest.json`) for metadata tracking and chain building

### Notes

- **Versionstamps are unique to each database instance**: When you restore a database from a dump, the restored database will have completely different versionstamps than the original database.
- **Incremental dumps from the original database can be applied to restored databases**: You can apply incremental dumps created from the original database onto a restored database as part of the restoration process.
- **Cannot continue incremental chains after restoration**: After restoring a database, you cannot continue taking incremental dumps based on the original database's versionstamps. To start a new incremental backup chain on a restored database:
  1. Take a new full dump of the restored database
  2. Use this new full dump as the base for future incremental dumps

For detailed design documentation including consistency guarantees and safety mechanisms, see [docs/design.md](docs/design.md).
