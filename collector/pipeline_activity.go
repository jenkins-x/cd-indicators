package collector

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/cd-indicators/store"
	jenkinsv1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	jxclientset "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
	informers "github.com/jenkins-x/jx-api/v4/pkg/client/informers/externalversions"
	"github.com/scylladb/go-set/strset"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

type PipelineActivityCollector struct {
	JXClient       *jxclientset.Clientset
	Namespace      string
	ResyncInterval time.Duration
	GitOwners      *strset.Set
	Store          *store.PipelineStore
	Logger         *logrus.Logger
}

func (c *PipelineActivityCollector) Start(ctx context.Context) error { // nolint: unparam
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		c.JXClient,
		c.ResyncInterval,
		informers.WithNamespace(c.Namespace),
	)
	informerFactory.Jenkins().V1().PipelineActivities().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pa := obj.(*jenkinsv1.PipelineActivity)
			c.storePipeline(pa)
		},
		UpdateFunc: func(old, new interface{}) {
			pa := new.(*jenkinsv1.PipelineActivity)
			c.storePipeline(pa)
		},
		DeleteFunc: func(obj interface{}) {
			pa := obj.(*jenkinsv1.PipelineActivity)
			c.storePipeline(pa)
		},
	})
	informerFactory.Start(ctx.Done())

	return nil
}
func SimplifyStep(coreStep jenkinsv1.CoreActivityStep) store.SimplifiedActivityStep {

	if coreStep.Status == "" || coreStep.StartedTimestamp == nil || coreStep.CompletedTimestamp == nil {
		return store.SimplifiedActivityStep{}
	}

	return store.SimplifiedActivityStep{
		Name:               coreStep.Name,
		Status:             coreStep.Status.String(),
		StartedTimestamp:   coreStep.StartedTimestamp.Time,
		CompletedTimestamp: coreStep.CompletedTimestamp.Time,
		Duration:           coreStep.CompletedTimestamp.Time.Sub(coreStep.StartedTimestamp.Time),
	}
}
func (c *PipelineActivityCollector) storePipeline(pa *jenkinsv1.PipelineActivity) {
	if pa == nil {
		return
	}

	log := c.Logger.WithField("pipeline", pa.Name)
	if !pa.Spec.Status.IsTerminated() {
		log.WithField("status", pa.Spec.Status).Trace("Ignoring PipelineActivity which is not terminated")
		return
	}
	if pa.Spec.StartedTimestamp == nil || pa.Spec.CompletedTimestamp == nil {
		log.Trace("Ignoring PipelineActivity which has no start or end time")
		return
	}
	if pa.Spec.Context == "" {
		log.Trace("Ignoring PipelineActivity with no context")
		return
	}
	if pa.Spec.GitRepository == "" {
		log.Trace("Ignoring PipelineActivity with no repository")
		return
	}
	if !c.GitOwners.IsEmpty() && !c.GitOwners.Has(pa.Spec.GitOwner) {
		log.
			WithField("owner", pa.Spec.GitOwner).
			WithField("allowed-owners", c.GitOwners.String()).
			Debug("Ignoring PipelineActivity with not-allowed git owner")
		return
	}

	var simplifiedSteps []store.SimplifiedActivityStep
	for _, step := range pa.Spec.Steps {
		log.WithField("step", step.Kind).Trace("Simplifying step")
		if step.Kind == "Stage" {
			for _, stageStep := range step.Stage.Steps {
				simplifiedStep := SimplifyStep(stageStep)
				if simplifiedStep.Name == "" {
					log.WithField("step", step.Kind).Trace("Ignoring empty step")
				} else {
					simplifiedSteps = append(simplifiedSteps, simplifiedStep)
				}
			}
			simplifiedSteps = append(simplifiedSteps, SimplifyStep(step.Stage.CoreActivityStep))
		}
		if step.Kind == "Promote" {
			simplifiedStep := SimplifyStep(step.Promote.CoreActivityStep)
			if simplifiedStep.Name == "" {
				log.WithField("step", step.Kind).Trace("Ignoring empty step")
			} else {
				simplifiedSteps = append(simplifiedSteps, simplifiedStep)
			}
		}
		if step.Kind == "Preview" {
			simplifiedStep := SimplifyStep(step.Preview.CoreActivityStep)
			if simplifiedStep.Name == "" {
				log.WithField("step", step.Kind).Trace("Ignoring empty step")
			} else {
				simplifiedSteps = append(simplifiedSteps, simplifiedStep)
			}
		}
	}
	log.WithField("steps", len(simplifiedSteps)).Trace("Simplified steps")
	pipeline := store.Pipeline{
		Owner:      pa.Spec.GitOwner,
		Repository: pa.Spec.GitRepository,
		Context:    pa.Spec.Context,
		Status:     string(pa.Spec.Status),
		Author:     pa.Spec.Author,
		StartTime:  pa.Spec.StartedTimestamp.Time.In(time.UTC),
		EndTime:    pa.Spec.CompletedTimestamp.Time.In(time.UTC),
		Steps:      simplifiedSteps,
	}
	pipeline.Duration = pipeline.EndTime.Sub(pipeline.StartTime)

	var err error
	if strings.HasPrefix(pa.Spec.GitBranch, "PR-") {
		pipeline.Type = store.PipelineTypePullRequest
		pipeline.PullRequest, err = strconv.Atoi(strings.TrimPrefix(pa.Spec.GitBranch, "PR-"))
		if err != nil {
			log.WithField("branch", pa.Spec.GitBranch).WithError(err).Error("Can't collect a PipelineActivity with an invalid Git branch field")
			return
		}
	} else {
		pipeline.Type = store.PipelineTypeRelease
	}

	pipeline.Build, err = strconv.Atoi(pa.Spec.Build)
	if err != nil {
		log.WithField("build", pa.Spec.Build).WithError(err).Error("Can't collect a PipelineActivity with an invalid build field")
		return
	}

	log.Debug("Storing pipeline")
	ctx := context.Background()
	err = c.Store.Add(ctx, pipeline)
	if err != nil {
		log.WithError(err).Error("Failed to store pipeline")
		return
	}
}
