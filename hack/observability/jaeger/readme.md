# Jaeger Operator

This directory holds the charts for deploying the Jaeger operator into a development cluster and enabling
tracing for the CAPZ controller running in KinD (`make tilt-up`).

```
./hack/observability/jaeger/
├── fetch-jaeger-resources.sh
├── kustomization.yaml
├── readme.md
├── resources
│   ├── all-in-one.yaml
│   ├── crds.yaml
│   ├── kustomization.yaml
│   ├── operator.yaml
│   ├── role_binding.yaml
│   ├── role.yaml
│   └── service_account.yaml
└── sidecar_injection.yaml
```

## Sidecar Injection
`sidecar_injection.yaml` is responsible for adding a Jaeger "All in One" container to run within the CAPZ
controller pod. It will collect traces and expose them via ingres on `http://localhost:8080`.

## Updating Resources
Occasionally, the resources in this directory will need to be updated. To update the resources with the latest
deployment, run `./fetch-jaeger-resources.sh`. This will fetch the latest resources and replace the existing
yaml definitions.