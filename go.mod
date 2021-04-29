module github.com/jenkins-x/cd-indicators

go 1.15

require (
	github.com/jackc/pgx/v4 v4.10.1
	github.com/jenkins-x/go-scm v1.7.3
	github.com/jenkins-x/jx-api/v4 v4.0.14
	github.com/jenkins-x/lighthouse v1.0.33
	github.com/mitchellh/go-homedir v1.1.0
	github.com/scylladb/go-set v1.0.2
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
)
