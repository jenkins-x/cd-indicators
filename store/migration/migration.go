package migration

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	migrationsTableName = "migrations"
)

type Func func(ctx context.Context, tx pgx.Tx) error

type Migratable interface {
	TableName() string
	Migrations() []Func
}

type Migrator struct {
	ConnPool *pgxpool.Pool
}

func (m *Migrator) Migrate(ctx context.Context, migratables ...Migratable) error {
	tx, err := m.ConnPool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	err = m.ensureMigrationsTableExists(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to ensure that the migrations table '%s' exists: %w", migrationsTableName, err)
	}

	_, err = tx.Exec(ctx, fmt.Sprintf("LOCK TABLE ONLY %s IN ACCESS EXCLUSIVE MODE", migrationsTableName))
	if err != nil {
		return fmt.Errorf("failed to lock the migrations table '%s': %w", migrationsTableName, err)
	}

	for _, migratable := range migratables {
		var currentMigrationLevel int
		err = tx.QueryRow(ctx, fmt.Sprintf("SELECT migration_level FROM %s WHERE table_name=$1;", migrationsTableName), migratable.TableName()).Scan(&currentMigrationLevel)
		if errors.Is(err, pgx.ErrNoRows) {
			currentMigrationLevel = 0
			err = nil
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve current migration level for table %s: %w", migratable.TableName(), err)
		}

		err = m.migrate(ctx, tx, migratable, currentMigrationLevel)
		if err != nil {
			return fmt.Errorf("failed to retrieve current migration level for table %s: %w", migratable.TableName(), err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit DB transaction: %w", err)
	}

	return nil
}

func (m *Migrator) migrate(ctx context.Context, tx pgx.Tx, migratable Migratable, currentMigrationLevel int) error {
	for i, migrationFunc := range migratable.Migrations() {
		migrationLevel := i + 1
		if migrationLevel <= currentMigrationLevel {
			continue
		}

		tx, err := tx.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to start a new DB transaction for table %s migration %d: %w", migratable.TableName(), migrationLevel, err)
		}
		defer tx.Rollback(ctx) // nolint: errcheck

		err = migrationFunc(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to run migration %d for table %s: %w", migrationLevel, migratable.TableName(), err)
		}

		ct, err := tx.Exec(ctx, fmt.Sprintf("INSERT INTO %s (table_name, migration_level) VALUES($1, $2) ON CONFLICT (table_name) DO UPDATE SET migration_level = EXCLUDED.migration_level;", migrationsTableName), migratable.TableName(), migrationLevel)
		if err != nil {
			return fmt.Errorf("failed to update migrations table for table %s and migration level %d: %w", migratable.TableName(), migrationLevel, err)
		}
		if ct.RowsAffected() != 1 {
			return fmt.Errorf("failed to update migrations table for table %s and migration level %d: unexpected result %s", migratable.TableName(), migrationLevel, ct.String())
		}

		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit DB transaction for table %s migration %d: %w", migratable.TableName(), migrationLevel, err)
		}
	}

	return nil
}

func (m *Migrator) ensureMigrationsTableExists(ctx context.Context, tx pgx.Tx) error {
	tx, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	migrationsTableExists := false
	err = tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1);", migrationsTableName).Scan(&migrationsTableExists)
	if err != nil {
		return fmt.Errorf("failed to check if table %s exists: %w", migrationsTableName, err)
	}

	if migrationsTableExists {
		return nil
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s (
			table_name VARCHAR NOT NULL,
			migration_level int NOT NULL,
			CONSTRAINT %[1]s_pkey PRIMARY KEY (table_name)
		);
	`, migrationsTableName))
	if err != nil {
		return fmt.Errorf("failed to create the table %s: %w", migrationsTableName, err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func ExecSQLFunc(sql string, arguments ...interface{}) Func {
	return func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sql, arguments...)
		return err
	}
}
