module github.com/jenkins-x/cd-indicators

go 1.15

require (
	cloud.google.com/go/storage v1.12.0 // indirect
	github.com/google/go-github/v33 v33.0.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/jackc/pgx/v4 v4.10.1
	github.com/jenkins-x/go-scm v1.6.5
	github.com/jenkins-x/jx-api/v4 v4.0.14
	github.com/jenkins-x/lighthouse v0.0.939
	github.com/mitchellh/go-homedir v1.1.0
	github.com/scylladb/go-set v1.0.2
	github.com/sirupsen/logrus v1.7.0
	golang.org/x/oauth2 v0.0.0-20201208152858-08078c50e5b5 // indirect
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
)
