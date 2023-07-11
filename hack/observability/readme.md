# Observability

During development of CAPZ, it's become clear log messages can only tell part of the debugging
story. As the code becomes more complex, we need better tools to help us understand what is
happening in the controller.

In that spirit, we've added support for metrics and tracing. This directory contains resources for
development clusters to install [OpenTelemetry](https://opentelemetry.io/),
[Jaeger](https://github.com/jaegertracing/jaeger-operator), and
[Prometheus](https://github.com/prometheus-operator/prometheus-operator). With

If you run `make tilt-up` to create your development cluster, these observability resources are
installed and enabled automatically.

<!-- markdown-link-check-disable-next-line -->
To access traces in the Jaeger web interface, visit http://localhost:16686/ or select the
"traces: jaeger-all-in-one" resource in the Tilt UI and click on "View traces" near the top of
the screen.

To access traces in
[Application Insights](https://learn.microsoft.com/azure/azure-monitor/app/app-insights-overview),
specify an `AZURE_INSTRUMENTATION_KEY` in your `tilt-settings.yaml`, then navigate to the
App Insights resource in the Azure Portal and choose "Transaction search" to query for traces. See
the tracing docs for more detail.

<!-- markdown-link-check-disable-next-line -->
To access metrics in the Prometheus web interface, visit http://localhost:9090/ or select the
"metrics: prometheus-operator" resource in the Tilt UI and click on "View metrics" near the top of
the screen.
