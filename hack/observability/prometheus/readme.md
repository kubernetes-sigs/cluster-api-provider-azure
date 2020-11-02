# Prometheus Metrics
This directory holds the resources for deploy prometheus and monitoring metrics coming from the CAPZ controller.
It adds a label to the CAPZ controller and scrapes metrics exposed on `:8080`. This is intended for development
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