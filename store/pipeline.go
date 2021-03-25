package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jenkins-x/cd-indicators/store/migration"
)

type PipelineType string

const (
	PipelineTypeRelease     = PipelineType("release")
	PipelineTypePullRequest = PipelineType("pullrequest")
)

type Pipeline struct {
	Type        PipelineType
	Owner       string
	Repository  string
	PullRequest int
	Context     string
	Build       int
	Status      string
	Author      string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

type PipelineStore struct {
	connPool *pgxpool.Pool
}

func (s *PipelineStore) TableName() string {
	return "pipelines"
}

func (s *PipelineStore) Migrations() []migration.Func {
	return []migration.Func{
		migration.ExecSQLFunc(`
			CREATE TABLE pipelines (
				type VARCHAR NOT NULL,
				owner VARCHAR NOT NULL,
				repository VARCHAR NOT NULL,
				pull_request int,
				context VARCHAR NOT NULL,
				build int NOT NULL,
				status VARCHAR NOT NULL,
				author VARCHAR,
				start_time timestamp without time zone NOT NULL,
				end_time timestamp without time zone NOT NULL,
				duration bigint NOT NULL,
				CONSTRAINT pipeline_pkey PRIMARY KEY (type, owner, repository, pull_request, context, build)
			);
		`),
	}
}

func (s *PipelineStore) Add(ctx context.Context, p Pipeline) error {
	tx, err := s.connPool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	_, err = tx.Exec(ctx, "INSERT INTO pipelines (type, owner, repository, pull_request, context, build, status, author, start_time, end_time, duration) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING;", p.Type, p.Owner, p.Repository, p.PullRequest, p.Context, p.Build, p.Status, p.Author, p.StartTime, p.EndTime, p.Duration.Seconds())
	if err != nil {
		return fmt.Errorf("failed to add pipeline: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit insertion of pipeline: %w", err)
	}

	return nil
}
