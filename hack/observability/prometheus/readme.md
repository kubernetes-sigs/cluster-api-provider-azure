# Prometheus Metrics
This directory holds the resources for deploy prometheus and monitoring metrics coming from the CAPZ controller.
It adds a label to the CAPZ controller and scrapes metrics via the kube-rbac-proxy sidecar. This is intended for development
purposes.

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
Once the Tilt cluster is up and running, run `kubectl port-forward -n capz-system prometheus-prometheus-0 9090` and
open `http://localhost:9090` to see the Prometheus UI.