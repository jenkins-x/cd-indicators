package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/tracelog"
	"net/http"
	"os"
	"strings"
	"time"

	logrusadapter "github.com/jackc/pgx-logrus"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jenkins-x/cd-indicators/collector"
	"github.com/jenkins-x/cd-indicators/internal/kube"
	"github.com/jenkins-x/cd-indicators/internal/lighthouse"
	"github.com/jenkins-x/cd-indicators/internal/version"
	"github.com/jenkins-x/cd-indicators/store"
	jxclientset "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
	"github.com/scylladb/go-set/strset"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	options struct {
		namespace           string
		resyncInterval      time.Duration
		gitOwners           []string
		postgresURI         string
		lighthouseHMACKey   string
		kubeConfigPath      string
		listenAddr          string
		logLevelForPostgres string
		logLevel            string
		printVersion        bool
	}
)

func init() {
	pflag.StringVar(&options.namespace, "namespace", "jx", "Name of the jx namespace")
	pflag.StringVar(&options.postgresURI, "postgres-uri", "postgres://localhost:5432/indicators", "URI of the postgres DB to connnect to")
	pflag.DurationVar(&options.resyncInterval, "resync-interval", 1*time.Hour, "Resync interval between full re-list operations")
	pflag.StringSliceVar(&options.gitOwners, "git-owners", []string{}, "List of git owners/organizations to collect indicators from. Leave empty to collect from all")
	pflag.StringVar(&options.lighthouseHMACKey, "lighthouse-hmac-key", os.Getenv("LIGHTHOUSE_HMAC_KEY"), "HMAC key used by Lighthouse to sign the webhooks")
	pflag.StringVar(&options.listenAddr, "listen-addr", ":8080", "Address on which the HTTP server will listen for incoming connections")
	pflag.StringVar(&options.logLevel, "log-level", "INFO", "Log level - one of: trace, debug, info, warn(ing), error, fatal or panic")
	pflag.StringVar(&options.logLevelForPostgres, "log-level-db", "WARN", "Log level for the database operations - one of: trace, debug, info, warn, error or none")
	pflag.StringVar(&options.kubeConfigPath, "kubeconfig", kube.DefaultKubeConfigPath(), "Kubernetes Config Path. Default: KUBECONFIG env var value")
	pflag.BoolVar(&options.printVersion, "version", false, "Print the version")
}

func main() {
	pflag.Parse()

	if options.printVersion {
		fmt.Printf("Version %s - Revision %s - Date %s", version.Version, version.Revision, version.Date)
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	logger := logrus.New()
	logLevel, err := logrus.ParseLevel(options.logLevel)
	if err != nil {
		logger.WithField("logLevel", options.logLevel).WithError(err).Error("Invalid log level")
	} else {
		logger.SetLevel(logLevel)
	}
	logger.WithField("logLevel", logLevel).Info("Starting")

	kConfig, err := kube.NewConfig(options.kubeConfigPath)
	if err != nil {
		logger.WithError(err).Fatal("failed to create a Kubernetes config")
	}
	jxClient, err := jxclientset.NewForConfig(kConfig)
	if err != nil {
		logger.WithError(err).Fatal("failed to create a Jenkins X client")
	}

	dbconf, err := pgxpool.ParseConfig(options.postgresURI)
	if err != nil {
		logger.WithError(err).Fatal("Failed to parse postgresURI")
	}
	pgLogLevel, err := tracelog.LogLevelFromString(strings.ToLower(options.logLevelForPostgres))
	if err != nil {
		logger.WithField("logLevel", strings.ToLower(options.logLevelForPostgres)).WithError(err).Fatal("Invalid log level for database operations")
	}
	dbconf.ConnConfig.Tracer = &tracelog.TraceLog{Logger: logrusadapter.NewLogger(logger), LogLevel: pgLogLevel}
	dbpool, err := pgxpool.NewWithConfig(ctx, dbconf)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer dbpool.Close()

	s, err := store.New(ctx, dbpool)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize the store")
	}

	lighthouseHandler := lighthouse.Handler{
		SecretToken: options.lighthouseHMACKey,
		Logger:      logger,
	}

	logger.WithField("namespace", options.namespace).WithField("resyncInterval", options.resyncInterval).Info("Starting Collector")
	err = (&collector.Collector{
		JXClient:          jxClient,
		Namespace:         options.namespace,
		ResyncInterval:    options.resyncInterval,
		GitOwners:         strset.New(options.gitOwners...),
		Store:             s,
		LighthouseHandler: &lighthouseHandler,
		Logger:            logger,
	}).Start(ctx)
	if err != nil {
		logger.WithError(err).Fatal("Failed to start the collector")
	}

	http.Handle("/lighthouse/events", &lighthouseHandler)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger.WithField("listenAddr", options.listenAddr).Info("Starting HTTP Server")
	err = http.ListenAndServe(options.listenAddr, nil)
	if !errors.Is(err, http.ErrServerClosed) {
		logger.WithError(err).Fatal("failed to start HTTP server")
	}
}
