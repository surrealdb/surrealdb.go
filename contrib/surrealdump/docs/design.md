# Surrealdump Design

`surrealdump` tries its best to provide consistent full and incremental dumps of the database
in a way so that it works on any SurrealDB backend.

Some SurrealDB backend's isolation model and SurrealDB's lack of versionstamp visibility in transactions make traditional point-in-time dumps not straight-forward.

> Note that SurrealDB's "versionstamp" is a concept related to "commit version" or "transaction ID" in other databases. A versionstamp is used by SurrealDB's "change feed" feature to serialize all the transactions to the database and tables on which the change feed is enabled.

`surrealdump` combines an inconsistent full dump with change feed data to guarantee consistency at a specific versionstamp.

More concretely, `surrealdump` create a consistent full dump that can be reliably followed by incremental dumps without missing any changes. And it does so by capturing an inconsistent full dump of all records alongside the complete change history from before the dump started, then replay changes over the inconsistent dump during restore.

## The Problem

Without serializable isolation and the commit version exposed to the transaction client, a full dump taken between time T1 and T2 may contain:
- Some records as they existed at T1
- Other records as they existed at T2
- Records modified during the dump in various intermediate states

If an incremental dump starts at T2, changes between T1 and T2 for the "T1 records" would be missed in the resulting database.

How can we avoid that?

## The Solution

We solve this consistency problem by capturing a complete change history alongside the inconsistent dump. During restore, we first apply the inconsistent dump, then replay the changes to bring all records to a consistent state at a specific versionstamp (vs_2). This ensures no data is lost between dumps.

### Full Dump Structure

The dump contains two parts:
1. **Inconsistent Full Dump** - All records dumped between versionstamp vs_1 and vs_2 (inherently inconsistent)
2. **Change History** - All changes from vs_0 (before dump) through vs_2 (after dump)

### Versionstamp Tracking

Since SurrealDB transactions don't return versionstamps, we use a temp table to capture:
- **vs_0**: Before starting the dump (baseline, for simplicity we cann this StartVersionstamp in code)
- **vs_1**: Before dumping records (inconsistent dump start, does not appear in code)
- **vs_2**: After dumping records (inconsistent dump end, we call this EndVersionstamp in code)

### Restore Process

1. Apply all records from the inconsistent full dump using UPSERT
2. Apply all changes from the incremental dumps in ascending order of versionstamps, up to vs_2
3. Result: Database state consistent at vs_2

## Why This Works

The inconsistent full dump might have record A at vs_1 and record B at vs_2, but the included changes from vs_0 to vs_2 ensure:
- Record A gets updated from its vs_1 state to vs_2
- Record B already at vs_2 remains unchanged (idempotent UPSERT)
- Any records created or deleted between vs_1 and vs_2 are handled by the changes

Subsequent incremental dumps starting at vs_2 will capture all future changes without gaps.

## Technical Considerations

- **Change feeds** must be enabled on all tables (automatically attempted by the dumper)
- **Backend isolation** varies (e.g., TiKV provides snapshot isolation), and further more SurrealDB transactions do not include versionstamps in the transaction result as of today
- **UPSERT operations** handle records appearing in both the inconsistent dump and changes without creating duplicates

## Dump Chains

To ensure data integrity and prevent corruption from incorrect dump application, surrealdump implements a dump chain system based on manifest files that track dump metadata and relationships among full and incremental dumps.

### Manifest Structure

Each dump automatically generates a `.manifest.json` file containing:
- **Dump type** (full or incremental)
- **Versionstamp ranges** (start and end versionstamps)
- **Start versionstamp** for incremental dumps (equals to the end versionstamp of the previous dump)
- **Namespace and database** information
- **SHA256 hash** for integrity verification
- **Timestamp and size** metadata

### Chain Building

The dump chain system provides several key features:

1. **ValidateChain**: Ensures incremental dumps connect properly without gaps in versionstamps
2. **CanApplyIncremental**: Verifies an incremental dump can be applied to the current database state
3. **BuildChains**: Automatically discovers and constructs valid dump chains from a directory
4. **ScanDirectory**: Finds all dumps and their manifests in a directory
5. **GetManifestsForVersionstamp**: Determines which dumps are needed to restore to a specific point

See the `ScanChains` function for more details.

### Safety Guards

The system prevents several types of errors that could lead to data corruption:

1. **Gap Prevention**: Refuses to apply incremental dumps when there's a gap in versionstamps
   - For example, you cannot apply an incremental dump expecting start versionstamp 300 to a database at versionstamp 200

2. **Wrong Start Detection**: Validates that incremental dumps are applied to the correct start state
   - Each incremental dump records its expected start versionstamp
   - Restoration fails if the current state doesn't match the expected start

3. **Namespace/Database Consistency**: Ensures dumps from different namespaces or databases aren't mixed
   - Manifest files track the source namespace and database
   - Chain building groups dumps by namespace/database combination

4. **Order Enforcement**: Guarantees dumps are applied in the correct sequence
   - Full dump must be applied first
   - Incremental dumps must be applied in versionstamp order
   - Point-in-time restore automatically determines the correct sequence

5. **Integrity Verification**: SHA256 hashes verify dump files haven't been corrupted
   - Each manifest includes the hash of its corresponding dump file
   - Restoration can verify file integrity before application

### Point-in-Time Recovery

The chain validation system enables point-in-time recovery by:
1. Scanning a directory for all available dumps
2. Building valid chains from full dumps and their incremental continuations
3. Validating chain consistency
4. Determining the minimum set of dumps needed for a target versionstamp
5. Applying dumps in the correct order to reach the desired state

This design ensures that even with distributed systems and eventual consistency, administrators can reliably backup and restore their SurrealDB databases without risk of data corruption or loss.
