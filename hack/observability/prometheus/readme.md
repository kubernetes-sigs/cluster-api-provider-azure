# Prometheus Metrics

This directory contains resources for deploying [prometheus](https://prometheus.io/) to query and
view metrics coming from the CAPZ controller. It adds a label to the CAPZ controller and scrapes
metrics via the kube-rbac-proxy sidecar. This is intended for development purposes only.

**NOTE:** This metrics pipeline is evolving and should not be considered for production usage.

```
./hack/observability/prometheus/
├── capz_prom_label.yaml
├── kustomization.yaml
├── readme.md
└── resources
    ├── monitor.yaml
    ├── prom_service.yaml
    ├── role_binding.yaml
    ├── role.yaml
    └── service_account.yaml
```

## To View Metrics

Once the Tilt cluster is up and running, select the
"[metrics: prometheus-operator](http://localhost:10350/r/metrics%3A%20prometheus-operator/overview)" <!-- markdown-link-check-disable-line -->
resource. Click on the "View metrics" link near the top of the screen to access the prometheus web
interface and query metrics data for your management cluster.
