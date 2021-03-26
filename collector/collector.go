package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/cd-indicators/internal/lighthouse"
	"github.com/jenkins-x/cd-indicators/store"
	jxclientset "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
	"github.com/scylladb/go-set/strset"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	JXClient          *jxclientset.Clientset
	Namespace         string
	ResyncInterval    time.Duration
	GitOwners         *strset.Set
	Store             *store.Store
	LighthouseHandler *lighthouse.Handler
	Logger            *logrus.Logger

	pipelineActivityCollector *PipelineActivityCollector
	releaseCollector          *ReleaseCollector
	pullRequestCollector      *PullRequestCollector
	deploymentCollector       *DeploymentCollector
}

func (c *Collector) Start(ctx context.Context) error {
	c.pipelineActivityCollector = &PipelineActivityCollector{
		JXClient:       c.JXClient,
		Namespace:      c.Namespace,
		ResyncInterval: c.ResyncInterval,
		GitOwners:      c.GitOwners,
		Store:          c.Store.Pipelines,
		Logger:         c.Logger,
	}
	c.releaseCollector = &ReleaseCollector{
		JXClient:       c.JXClient,
		Namespace:      c.Namespace,
		ResyncInterval: c.ResyncInterval,
		GitOwners:      c.GitOwners,
		Store:          c.Store.Releases,
		Logger:         c.Logger,
	}
	c.pullRequestCollector = &PullRequestCollector{
		GitOwners:         c.GitOwners,
		Store:             c.Store.PullRequests,
		LighthouseHandler: c.LighthouseHandler,
		Logger:            c.Logger,
	}
	c.deploymentCollector = &DeploymentCollector{
		GitOwners:         c.GitOwners,
		Store:             c.Store.Deployments,
		LighthouseHandler: c.LighthouseHandler,
		Logger:            c.Logger,
	}

	if err := c.pipelineActivityCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PipelineActivity Collector: %w", err)
	}
	if err := c.releaseCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Release Collector: %w", err)
	}
	if err := c.pullRequestCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PullRequest Collector: %w", err)
	}
	if err := c.deploymentCollector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Deployment Collector: %w", err)
	}

	return nil
}
