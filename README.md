# Continuous Delivery Indicators for JayeX

It is composed of:
- a collector, written in Go, which:
  - watches the JayeX Pipeline Activities in the Kubernetes Cluster
  - watches the JayeX Releases in the Kubernetes Cluster & from Lighthouse events
  - watches the Pull Request Events from Lighthouse
  - watches the Deployment Events from Lighthouse
- a storage: a PostgreSQL database
- a visualizer: Grafana
  - the grafana dashboards are stored in charts/cd-indicators/grafana-dashboards
