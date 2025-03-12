package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jenkins-x/cd-indicators/store/migration"
)

type Deployment struct {
	Owner          string
	Repository     string
	Version        string
	Environment    string
	DeploymentTime time.Time
}

func (d Deployment) String() string {
	return fmt.Sprintf(`"%s/%s" v %q in %q`, d.Owner, d.Repository, d.Version, d.Environment)
}

type DeploymentStore struct {
	connPool *pgxpool.Pool
}

func (s *DeploymentStore) TableName() string {
	return "deployments"
}

func (s *DeploymentStore) Migrations() []migration.Func {
	return []migration.Func{
		migration.ExecSQLFunc(`
			CREATE TABLE deployments (
				owner VARCHAR NOT NULL,
				repository VARCHAR NOT NULL,
				version VARCHAR NOT NULL,
				environment VARCHAR NOT NULL,
				deployment_time timestamp without time zone,
				CONSTRAINT deployments_pkey PRIMARY KEY (owner, repository, version, environment)
			);
		`),
	}
}

func (s *DeploymentStore) Add(ctx context.Context, d Deployment) error {
	tx, err := s.connPool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	_, err = tx.Exec(ctx, `
	INSERT INTO deployments (owner, repository, version, environment, deployment_time) 
	VALUES ($1, $2, $3, $4, $5) 
	ON CONFLICT ON CONSTRAINT deployments_pkey DO NOTHING;`,
		d.Owner, d.Repository, d.Version, d.Environment, d.DeploymentTime)
	if err != nil {
		return fmt.Errorf("failed to add deployment: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit insertion of deployment: %w", err)
	}

	return nil
}
