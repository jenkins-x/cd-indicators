# Continuous Delivery Indicators for Jenkins X

It is composed of:
- a collector, written in Go, which:
  - watches the Jenkins X Pipeline Activities in the Kubernetes Cluster
  - watches the Jenkins X Releases in the Kubernetes Cluster
  - watches the Pull Request Events from Lighthouse
- a storage: a PostgreSQL database
- a visualizer: Grafana
  - the grafana dashboards are stored in charts/cd-indicators/grafana-dashboards
