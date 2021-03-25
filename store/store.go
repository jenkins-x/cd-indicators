package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jenkins-x/cd-indicators/store/migration"
)

type Store struct {
	Pipelines    *PipelineStore
	PullRequests *PullRequestStore
	Releases     *ReleaseStore
}

func New(ctx context.Context, connPool *pgxpool.Pool) (*Store, error) {
	store := &Store{
		Pipelines: &PipelineStore{
			connPool: connPool,
		},
		PullRequests: &PullRequestStore{
			connPool: connPool,
		},
		Releases: &ReleaseStore{
			connPool: connPool,
		},
	}

	err := (&migration.Migrator{
		ConnPool: connPool,
	}).Migrate(ctx,
		store.Pipelines,
		store.PullRequests,
		store.Releases,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run store migrations: %w", err)
	}

	return store, nil
}
