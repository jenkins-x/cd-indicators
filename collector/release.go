package collector

import (
	"context"
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

type ReleaseCollector struct {
	JXClient       *jxclientset.Clientset
	Namespace      string
	ResyncInterval time.Duration
	GitOwners      *strset.Set
	Store          *store.ReleaseStore
	Logger         *logrus.Logger
}

func (c *ReleaseCollector) Start(ctx context.Context) error {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		c.JXClient,
		c.ResyncInterval,
		informers.WithNamespace(c.Namespace),
	)
	informerFactory.Jenkins().V1().Releases().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r := obj.(*jenkinsv1.Release)
			c.storeRelease(r)
		},
		UpdateFunc: func(old, new interface{}) {
			r := new.(*jenkinsv1.Release)
			c.storeRelease(r)
		},
		DeleteFunc: func(obj interface{}) {
			r := obj.(*jenkinsv1.Release)
			c.storeRelease(r)
		},
	})
	informerFactory.Start(ctx.Done())

	return nil
}

func (c *ReleaseCollector) storeRelease(r *jenkinsv1.Release) {
	if r == nil {
		return
	}

	log := c.Logger.WithField("release", r.Name)
	if r.Spec.GitOwner == "" || r.Spec.GitRepository == "" {
		log.Trace("Ignoring Release with no Git owner and/or repository")
		return
	}
	if !c.GitOwners.IsEmpty() && !c.GitOwners.Has(r.Spec.GitOwner) {
		log.
			WithField("owner", r.Spec.GitOwner).
			WithField("allowed-owners", c.GitOwners.String()).
			Debug("Ignoring Release with not-allowed git owner")
		return
	}

	contributors := strset.New()
	for _, commit := range r.Spec.Commits {
		if login := extractUserLogin(commit.Author); login != "" {
			contributors.Add(login)
		}
		if login := extractUserLogin(commit.Committer); login != "" {
			contributors.Add(login)
		}
	}
	for _, pr := range r.Spec.PullRequests {
		if login := extractUserLogin(pr.User); login != "" {
			contributors.Add(login)
		}
		if login := extractUserLogin(pr.ClosedBy); login != "" {
			contributors.Add(login)
		}
	}

	release := store.Release{
		Owner:        r.Spec.GitOwner,
		Repository:   r.Spec.GitRepository,
		Version:      strings.TrimPrefix(r.Spec.Version, "v"),
		Contributors: contributors.List(),
		ReleaseTime:  r.CreationTimestamp.Time.In(time.UTC),
	}

	log.Debug("Storing release")
	ctx := context.Background()
	err := c.Store.Add(ctx, release)
	if err != nil {
		log.WithError(err).Error("Failed to store release")
		return
	}
}

func extractUserLogin(user *jenkinsv1.UserDetails) string {
	if user == nil {
		return ""
	}

	if user.Login != "" {
		return user.Login
	}

	return ""
}
