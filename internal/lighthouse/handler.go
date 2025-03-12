package lighthouse

import (
	"net/http"

	"github.com/jenkins-x/go-scm/scm"
	lhv1alpha1 "github.com/jenkins-x/lighthouse-client/pkg/apis/lighthouse/v1alpha1"
	lhutil "github.com/jenkins-x/lighthouse-client/pkg/util"
	"github.com/sirupsen/logrus"
)

type WebhookHandlerFunc func(scm.Webhook) error

type ActivityHandlerFunc func(*lhv1alpha1.ActivityRecord) error

type Handler struct {
	SecretToken string
	Logger      *logrus.Logger

	webhookHandlers  []WebhookHandlerFunc
	activityHandlers []ActivityHandlerFunc
}

func (h *Handler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.Logger.WithField("method", r.Method).Debug("Invalid http method so returning 200")
		return
	}

	log := h.Logger.
		WithField("type", r.Header.Get(lhutil.LighthousePayloadTypeHeader)).
		WithField("kind", r.Header.Get(lhutil.LighthouseWebhookKindHeader)).
		WithField("UA", r.Header.Get("User-Agent"))

	webhook, activity, err := lhutil.ParseExternalPluginEvent(r, h.SecretToken)
	if err != nil {
		log.
			WithField("signature", r.Header.Get(lhutil.LighthouseSignatureHeader)).
			WithError(err).Error("Failed to parse lighthouse event")
		return
	}
	if webhook == nil && activity == nil {
		log.Error("Lighthouse event was empty: no webhook or activity")
		return
	}

	if webhook != nil {
		log := log.WithField("repo", webhook.Repository().FullName)
		log.Trace("Handling webhook")
		for _, handler := range h.webhookHandlers {
			err = handler(webhook)
			if err != nil {
				log.WithError(err).Error("Failed to process webhook")
			}
		}
	}
	if activity != nil {
		log := log.WithField("activity", activity.Name)
		log.Trace("Handling activity")
		for _, handler := range h.activityHandlers {
			err = handler(activity)
			if err != nil {
				log.WithError(err).Error("Failed to process activity")
			}
		}
	}
}

func (h *Handler) RegisterWebhookHandler(handler WebhookHandlerFunc) {
	h.webhookHandlers = append(h.webhookHandlers, handler)
}

func (h *Handler) RegisterActivityHandler(handler ActivityHandlerFunc) {
	h.activityHandlers = append(h.activityHandlers, handler)
}
