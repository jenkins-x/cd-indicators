package collector

import (
	"context"
	"time"

	"github.com/jenkins-x/cd-indicators/internal/lighthouse"
	"github.com/jenkins-x/cd-indicators/store"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/scylladb/go-set/strset"
	"github.com/sirupsen/logrus"
)

type PullRequestCollector struct {
	GitOwners         *strset.Set
	Store             *store.PullRequestStore
	LighthouseHandler *lighthouse.Handler
	Logger            *logrus.Logger
}

func (c *PullRequestCollector) Start(_ context.Context) error { // nolint: unparam
	c.LighthouseHandler.RegisterWebhookHandler(c.handleWebhook)
	return nil
}

func (c *PullRequestCollector) handleWebhook(webhook scm.Webhook) error {
	log := c.Logger.WithField("repo", webhook.Repository().FullName)

	switch event := webhook.(type) {

	// https://docs.github.com/en/developers/webhooks-and-events/webhook-events-and-payloads#pull_request
	case *scm.PullRequestHook:
		log := log.WithField("pr", event.PullRequest.Number).WithField("action", event.Action)
		switch event.Action {
		case scm.ActionReadyForReview, scm.ActionConvertedToDraft, scm.ActionMerge:
		case scm.ActionOpen:
			log = log.WithField("draft", event.PullRequest.Draft)
		case scm.ActionClose:
			log = log.WithField("merged", event.PullRequest.Merged)
		case scm.ActionLabel, scm.ActionUnlabel:
			log = log.WithField("label", event.Label.Name)
		default:
			log.Debug("Ignoring pullrequest hook event for this action")
			return nil
		}
		log.Debug("Handling pullrequest hook event")
		return c.storePullRequest(event.PullRequest, event.Action, scm.Review{}, event.Label)

	// https://docs.github.com/en/developers/webhooks-and-events/webhook-events-and-payloads#pull_request_review
	case *scm.ReviewHook:
		log := log.WithField("pr", event.PullRequest.Number).WithField("action", event.Action).WithField("reviewer", event.Review.Author.Login)
		switch event.Action {
		case scm.ActionSubmitted:
		default:
			log.Debug("Ignoring pullrequest review hook event for this action")
			return nil
		}
		log.Debug("Handling pullrequest review hook event")
		return c.storePullRequest(event.PullRequest, event.Action, event.Review, scm.Label{})

	default:
		log.Trace("Ignoring non pullrequest hook event")
	}

	return nil
}

func (c *PullRequestCollector) storePullRequest(pullRequest scm.PullRequest, action scm.Action, review scm.Review, label scm.Label) error {
	if !c.GitOwners.IsEmpty() && !c.GitOwners.Has(pullRequest.Repository().Namespace) {
		c.Logger.
			WithField("owner", pullRequest.Repository().Namespace).
			WithField("allowed-owners", c.GitOwners.String()).
			Debug("Ignoring PullRequest with not-allowed git owner")
		return nil
	}
	var (
		ctx = context.Background()
		now = time.Now()
		pr  = store.PullRequest{
			Owner:       pullRequest.Repository().Namespace,
			Repository:  pullRequest.Repository().Name,
			PullRequest: pullRequest.Number,
			Author:      pullRequest.Author.Login,
			State:       pullRequest.State,
		}
	)

	switch action {
	case scm.ActionOpen:
		pr.CreationTime = &pullRequest.Created
		if !pullRequest.Draft {
			pr.ReadyForReviewTime = &pullRequest.Created
		}
	case scm.ActionReadyForReview:
		pr.ReadyForReviewTime = &now
	case scm.ActionConvertedToDraft:
		// use a "zero" time to reset it
		pr.ReadyForReviewTime = new(time.Time)
	case scm.ActionLabel:
		if label.Name == "approved" {
			pr.ApprovedTime = &now
		}
	case scm.ActionUnlabel:
		if label.Name == "approved" {
			// use a "zero" time to reset it
			pr.ApprovedTime = new(time.Time)
		}
	case scm.ActionSubmitted:
		pr.Reviews++
		pr.Reviewers = append(pr.Reviewers, review.Author.Login)
	case scm.ActionMerge:
		pr.MergedTime = &now
	case scm.ActionClose:
		if pullRequest.Merged {
			pr.MergedTime = &now
		}
	}
	pr.CalculateDurations()

	c.Logger.WithField("pullrequest", pr.String()).Debug("Storing pullrequest")
	return c.Store.Add(ctx, pr)
}
