# Observability
During development of CAPZ, it has become clear log messages can only tell part of the debugging story. As complexity 
of the controller increases, we need better tools to help us understand what is happening in the controller.
In that spirit, we've added support for metrics and tracing. This directory provides tools for running development
clusters with [Jaeger](https://github.com/jaegertracing/jaeger-operator) and [Prometheus](https://github.com/prometheus-operator/prometheus-operator).

To access Jaeger traces, run `make tilt-up` and open `http://localhost:8080`.

To access Prometheus metrics, run `make tilt-up`, `kubectl port-forward -n prometheus prometheus-prometheus-0 9090:9090`,
and open `https://localhost:9090`.