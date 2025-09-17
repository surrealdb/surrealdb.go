package surrealnote

import (
	"context"
	"fmt"
	"log"
)

// Migrate performs schema migration operations on the configured database stores.
// This method initializes or updates the database schema to match the application's data model.
// It is designed for schema setup and updates, not for migrating data between stores.
//
// Migrate should be run before starting the application or after making changes to the data model.
// It operates on whatever store configuration is active (single PostgreSQL, single SurrealDB, or CQRS dual-store).
//
// For CQRS configurations, Migrate runs schema migrations on both the primary and secondary stores
// to ensure they have identical schema definitions. This is essential for maintaining data consistency
// during the migration process.
//
// The migration process:
//   - For PostgreSQL: Uses GORM AutoMigrate to create/update tables, indexes, and constraints
//   - For SurrealDB: Schema creation is implicit (tables are created automatically when data is inserted)
//   - For CQRS: Runs migrations on both stores sequentially, failing if either store migration fails
//
// This method is safe to run multiple times - it only creates missing schema elements and
// updates existing ones as needed. It does not drop or modify existing data.
//
// Returns an error if schema migration fails on any configured store.
func (a *App) Migrate(ctx context.Context, cmd *MigrateCommand) error {
	log.Println("Running database migrations...")
	if err := a.store.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Migrations completed successfully")
	return nil
}
