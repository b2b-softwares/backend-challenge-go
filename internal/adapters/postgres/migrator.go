package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/migrations"
)

type Migrator struct {
	pool *pgxpool.Pool
}

func NewMigrator(
	pool *pgxpool.Pool,
) *Migrator {
	return &Migrator{
		pool: pool,
	}
}

func (m *Migrator) Run(
	ctx context.Context,
) error {
	if m == nil || m.pool == nil {
		return fmt.Errorf(
			"migration runner requires a database pool",
		)
	}

	if err := m.createSchemaMigrationsTable(ctx); err != nil {
		return err
	}

	files, err := migrationFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf(
			"no embedded database migrations found",
		)
	}

	for _, file := range files {
		version, err := migrationVersion(file)
		if err != nil {
			return fmt.Errorf(
				"parse migration version %q: %w",
				file,
				err,
			)
		}

		applied, err := m.isApplied(
			ctx,
			version,
		)
		if err != nil {
			return fmt.Errorf(
				"check migration %q: %w",
				file,
				err,
			)
		}

		if applied {
			continue
		}

		if err := m.apply(
			ctx,
			version,
			file,
		); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) createSchemaMigrationsTable(
	ctx context.Context,
) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL
)
`

	if _, err := m.pool.Exec(
		ctx,
		query,
	); err != nil {
		return fmt.Errorf(
			"create schema_migrations table: %w",
			err,
		)
	}

	return nil
}

func (m *Migrator) isApplied(
	ctx context.Context,
	version int64,
) (bool, error) {
	const query = `
SELECT EXISTS (
    SELECT 1
    FROM schema_migrations
    WHERE version = $1
)
`

	var applied bool

	if err := m.pool.QueryRow(
		ctx,
		query,
		version,
	).Scan(&applied); err != nil {
		return false, err
	}

	return applied, nil
}

func (m *Migrator) apply(
	ctx context.Context,
	version int64,
	file string,
) error {
	sqlBytes, err := fs.ReadFile(
		migrations.FS,
		file,
	)
	if err != nil {
		return fmt.Errorf(
			"read migration %q: %w",
			file,
			err,
		)
	}

	sql := strings.TrimSpace(
		string(sqlBytes),
	)

	if sql == "" {
		return fmt.Errorf(
			"migration %q is empty",
			file,
		)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf(
			"begin migration %q: %w",
			file,
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(
		ctx,
		sql,
	); err != nil {
		return fmt.Errorf(
			"execute migration %q: %w",
			file,
			err,
		)
	}

	const insertQuery = `
INSERT INTO schema_migrations (
    version,
    name,
    applied_at
)
VALUES ($1, $2, $3)
`

	if _, err := tx.Exec(
		ctx,
		insertQuery,
		version,
		file,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf(
			"record migration %q: %w",
			file,
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"commit migration %q: %w",
			file,
			err,
		)
	}

	return nil
}

func migrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(
		migrations.FS,
		".",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"read embedded migrations: %w",
			err,
		)
	}

	files := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasSuffix(
			name,
			".up.sql",
		) {
			continue
		}

		files = append(
			files,
			name,
		)
	}

	sort.Slice(
		files,
		func(i, j int) bool {
			leftVersion, leftErr :=
				migrationVersion(files[i])

			rightVersion, rightErr :=
				migrationVersion(files[j])

			if leftErr != nil ||
				rightErr != nil {
				return files[i] < files[j]
			}

			return leftVersion < rightVersion
		},
	)

	return files, nil
}

func migrationVersion(
	file string,
) (int64, error) {
	base := filepath.Base(file)

	parts := strings.SplitN(
		base,
		"_",
		2,
	)

	if len(parts) != 2 {
		return 0, fmt.Errorf(
			"invalid migration filename",
		)
	}

	version, err := strconv.ParseInt(
		parts[0],
		10,
		64,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid version prefix: %w",
			err,
		)
	}

	if version <= 0 {
		return 0, fmt.Errorf(
			"migration version must be greater than zero",
		)
	}

	return version, nil
}
