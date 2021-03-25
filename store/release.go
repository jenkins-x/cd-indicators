package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jenkins-x/cd-indicators/store/migration"
)

type Release struct {
	Owner        string
	Repository   string
	Version      string
	Contributors []string
	ReleaseTime  time.Time
}

func (r Release) String() string {
	return fmt.Sprintf(`"%s/%s" %q`, r.Owner, r.Repository, r.Version)
}

type ReleaseStore struct {
	connPool *pgxpool.Pool
}

func (s *ReleaseStore) TableName() string {
	return "releases"
}

func (s *ReleaseStore) Migrations() []migration.Func {
	return []migration.Func{
		migration.ExecSQLFunc(`
			CREATE TABLE releases (
				owner VARCHAR NOT NULL,
				repository VARCHAR NOT NULL,
				version VARCHAR,
				contributors VARCHAR[],
				release_time timestamp without time zone NOT NULL,
				CONSTRAINT releases_pkey PRIMARY KEY (owner, repository, version)
			);
		`),
	}
}

func (s *ReleaseStore) Add(ctx context.Context, r Release) error {
	tx, err := s.connPool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	_, err = tx.Exec(ctx, "INSERT INTO releases (owner, repository, version, contributors, release_time) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;", r.Owner, r.Repository, r.Version, r.Contributors, r.ReleaseTime)
	if err != nil {
		return fmt.Errorf("failed to add release: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit insertion of release: %w", err)
	}

	return nil
}
