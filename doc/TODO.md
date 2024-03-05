# TODO

- **we are reporting the size and time in `io.instruments.go` as both a counter and a histogram**
  (count can be extracted from histogram in all "backends": prometheus, datadog, new relic, etc..?)

- allow different formats for the propagation of the trace (the `TextMapPropagator`)
    in [endpoint.go](../router/gin/endpoint.go) and in [state.go](../state/state.go).

- review how we pass the state.

- we cannot use `skipPaths` with the same value that we have to define the endpoints
    at the global layer, because we cannot know the matched pattern

- clean shutdown

- add configuration options to allow to use grpc credentials

- allow to tweak the `bucket` limits for different histograms (like 
    latency and size) 
    - `http/client/transport_metrics.go`: `timeBucketsOpt`, `sizeBucketsOpt`
    - `http/server/metrics.go`: `timeBucketsOpt`, `sizeBucketsOpt`
    - `io/instrumens.go`: `timeBucketsOpt`, `sizeBucketsOpt`
    - `lura/proxy.go`: timeBucketsOpt, sizeBucketsOpt

- review all metric instruments and procive the correct `metric.WithUnit` value.

# TO DECIDE

- do we want to have the `service_name` to override the global ServiceConfig name for
    the KrakenD gateway ?

- in `exporter/prometheus/prometheus.go` we could add a config option that will
    add a prefix to all reported metrics (using the `WithNamespace` option).

- in `lura/backend.go` and `lura/proxy.go` we are setting the static attrs, 
  and using `semconv.ServerAddress`, to set the concatenated list of hosts.
  Is that something that we want ? 
  
# TO CHECK

- in `exporter/otelcollector/otelcollector.go`, the `WithEndpoint` states that 
  **no http** schema should be  in cluded (nor path). To use `http` reporting methods, 
  the `WithInsecure` option should be used (and to user a path `WithURLPath`).
  Check that current implementation works as expected, or fix it.

- in `http/client/transport.go` we have commented out the `StartOptions` for 
    trace: review if would be useful to expose that in the config.

# Known Issues

There is an issue, that we might have already started an span at
the global layer, because we do not know if that path had to be ignored.
So, at the global layer, there is no way to skip the paths
