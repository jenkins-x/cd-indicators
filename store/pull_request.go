package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jenkins-x/cd-indicators/store/migration"
	"github.com/scylladb/go-set/strset"
)

type PullRequest struct {
	Owner              string
	Repository         string
	PullRequest        int
	Author             string
	State              string
	Reviews            int
	Reviewers          []string
	CreationTime       *time.Time
	ReadyForReviewTime *time.Time
	ApprovedTime       *time.Time
	TimeToReview       time.Duration
	MergedTime         *time.Time
	TimeToMerge        time.Duration
}

func (pr *PullRequest) MergeWith(other PullRequest) {
	pr.Reviews += other.Reviews
	pr.Reviewers = strset.Union(
		strset.New(pr.Reviewers...),
		strset.New(other.Reviewers...),
	).List()
	if pr.CreationTime == nil && other.CreationTime != nil {
		pr.CreationTime = other.CreationTime
	}
	if pr.ReadyForReviewTime == nil && other.ReadyForReviewTime != nil {
		pr.ReadyForReviewTime = other.ReadyForReviewTime
	}
	if pr.ReadyForReviewTime != nil && pr.ReadyForReviewTime.IsZero() {
		pr.ReadyForReviewTime = nil // force a reset
	}
	if pr.ApprovedTime == nil && other.ApprovedTime != nil {
		pr.ApprovedTime = other.ApprovedTime
	}
	if pr.ApprovedTime != nil && pr.ApprovedTime.IsZero() {
		pr.ApprovedTime = nil // force a reset
	}
	if pr.MergedTime == nil && other.MergedTime != nil {
		pr.MergedTime = other.MergedTime
	}
}

func (pr *PullRequest) CalculateDurations() {
	if pr.ReadyForReviewTime != nil && pr.ApprovedTime != nil {
		pr.TimeToReview = pr.ApprovedTime.Sub(*pr.ReadyForReviewTime)
	}
	if pr.ApprovedTime != nil && pr.MergedTime != nil {
		pr.TimeToMerge = pr.MergedTime.Sub(*pr.ApprovedTime)
	}
	if pr.TimeToMerge > 0 && pr.TimeToReview == 0 {
		pr.TimeToReview = pr.TimeToMerge // if it has never been approved, but forced-merge
	}
}

func (pr PullRequest) String() string {
	return fmt.Sprintf(`"%s/%s" #%v by %q`, pr.Owner, pr.Repository, pr.PullRequest, pr.Author)
}

type PullRequestStore struct {
	connPool *pgxpool.Pool
}

func (s *PullRequestStore) TableName() string {
	return "pull_requests"
}

func (s *PullRequestStore) Migrations() []migration.Func {
	return []migration.Func{
		migration.ExecSQLFunc(`
			CREATE TABLE pull_requests (
				owner VARCHAR NOT NULL,
				repository VARCHAR NOT NULL,
				pull_request int NOT NULL,
				author VARCHAR,
				state VARCHAR,
				reviews int,
				reviewers VARCHAR[],
				creation_time timestamp without time zone,
				ready_for_review_time timestamp without time zone,
				approved_time timestamp without time zone,
				time_to_review bigint,
				merged_time timestamp without time zone,
				time_to_merge bigint,
				CONSTRAINT pull_requests_pkey PRIMARY KEY (owner, repository, pull_request)
			);
		`),
	}
}

func (s *PullRequestStore) Add(ctx context.Context, pr PullRequest) error {
	tx, err := s.connPool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("failed to start a new DB transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint: errcheck

	var prFromDB PullRequest
	err = tx.QueryRow(ctx, fmt.Sprintf(`
	SELECT creation_time, ready_for_review_time, approved_time, merged_time, reviews, reviewers 
	FROM %s WHERE owner=$1 AND repository=$2 AND pull_request=$3`, s.TableName()), pr.Owner, pr.Repository, pr.PullRequest).Scan(
		&prFromDB.CreationTime,
		&prFromDB.ReadyForReviewTime,
		&prFromDB.ApprovedTime,
		&prFromDB.MergedTime,
		&prFromDB.Reviews,
		&prFromDB.Reviewers,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to retrieve current pullrequest %s: %w", pr, err)
	}
	if err == nil {
		pr.MergeWith(prFromDB)
	}
	pr.CalculateDurations()

	_, err = tx.Exec(ctx, `
	INSERT INTO pull_requests (owner, repository, pull_request, author, state, creation_time, ready_for_review_time, approved_time, time_to_review, merged_time, time_to_merge, reviews, reviewers) 
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) 
	ON CONFLICT ON CONSTRAINT pull_requests_pkey DO UPDATE 
	SET state = EXCLUDED.state, creation_time = EXCLUDED.creation_time, ready_for_review_time = EXCLUDED.ready_for_review_time, approved_time = EXCLUDED.approved_time, time_to_review = EXCLUDED.time_to_review, merged_time = EXCLUDED.merged_time, time_to_merge = EXCLUDED.time_to_merge, reviews = EXCLUDED.reviews, reviewers = EXCLUDED.reviewers;`,
		pr.Owner, pr.Repository, pr.PullRequest, pr.Author, pr.State, pr.CreationTime, pr.ReadyForReviewTime, pr.ApprovedTime, pr.TimeToReview.Seconds(), pr.MergedTime, pr.TimeToMerge.Seconds(), pr.Reviews, pr.Reviewers)
	if err != nil {
		return fmt.Errorf("failed to add pullrequest: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit insertion of pullrequest: %w", err)
	}

	return nil
}
