package collector

import (
	"context"
	"strings"

	"github.com/jenkins-x/cd-indicators/internal/lighthouse"
	"github.com/jenkins-x/cd-indicators/store"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/scylladb/go-set/strset"
	"github.com/sirupsen/logrus"
)

type DeploymentCollector struct {
	GitOwners         *strset.Set
	Store             *store.DeploymentStore
	LighthouseHandler *lighthouse.Handler
	Logger            *logrus.Logger
}

func (c *DeploymentCollector) Start(_ context.Context) error { // nolint: unparam
	c.LighthouseHandler.RegisterWebhookHandler(c.handleWebhook)
	return nil
}

func (c *DeploymentCollector) handleWebhook(webhook scm.Webhook) error {
	log := c.Logger.WithField("repo", webhook.Repository().FullName)

	switch event := webhook.(type) {
	case *scm.DeploymentStatusHook:
		log.WithField("environment", event.Deployment.Environment).WithField("ref", event.Deployment.Ref).WithField("state", event.DeploymentStatus.State).Debug("Handling deployment hook event")
		return c.storeDeployment(event.Deployment, event.DeploymentStatus)
	default:
		log.Trace("Ignoring non deployment hook event")
	}

	return nil
}

func (c *DeploymentCollector) storeDeployment(deployment scm.Deployment, status scm.DeploymentStatus) error {
	if !c.GitOwners.IsEmpty() && !c.GitOwners.Has(deployment.Namespace) {
		c.Logger.
			WithField("owner", deployment.Namespace).
			WithField("allowed-owners", c.GitOwners.String()).
			Debug("Ignoring Deployment with not-allowed git owner")
		return nil
	}

	d := store.Deployment{
		Owner:          deployment.Namespace,
		Repository:     deployment.Name,
		Version:        strings.TrimPrefix(deployment.Ref, "v"),
		Environment:    deployment.Environment,
		DeploymentTime: status.Created,
	}

	c.Logger.WithField("deployment", d.String()).Debugf("Storing deployment %#v", d)
	ctx := context.Background()
	return c.Store.Add(ctx, d)
}
