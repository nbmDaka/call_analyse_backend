package database

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// RunMigrations applies each embedded up migration once, in lexical version order.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS) error {
	if pool == nil {
		return fmt.Errorf("database pool is required")
	}

	if _, err := pool.Exec(ctx, migrationTableSQL); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	files, err := fs.Glob(migrationFS, "*.up.sql")
	if err != nil {
		return fmt.Errorf("list migration files: %w", err)
	}
	sort.Strings(files)

	for _, filename := range files {
		if err := runMigration(ctx, pool, migrationFS, filename); err != nil {
			return err
		}
	}

	return nil
}

func runMigration(ctx context.Context, pool *pgxpool.Pool, migrationFS fs.FS, filename string) error {
	version := strings.TrimSuffix(path.Base(filename), ".up.sql")
	if version == "" || version == path.Base(filename) {
		return fmt.Errorf("invalid migration filename %q", filename)
	}

	sql, err := fs.ReadFile(migrationFS, filename)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", filename, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", version, err)
	}
	defer tx.Rollback(ctx)

	var alreadyApplied bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&alreadyApplied); err != nil {
		return fmt.Errorf("check migration %q: %w", version, err)
	}
	if alreadyApplied {
		return nil
	}

	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("apply migration %q: %w", version, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return fmt.Errorf("record migration %q: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %q: %w", version, err)
	}

	return nil
}
