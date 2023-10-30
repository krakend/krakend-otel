# krakend-otel
KrakenD component for OpenTelemetry

**While we still need to start to implement OpenTelemetry natively in this repo**, doing a bridge is **already possible** today.

For instance:

`otel-collector.yaml`:
```yaml
receivers:
  opencensus:

exporters:
  logging:
    verbosity: detailed

  azuremonitor:
    maxbatchsize: 100
    maxbatchinterval: 10s
    # REPLACE WITH YOUR KEY:
    instrumentation_key: YOUR-KEY-HERE

service:
  telemetry:
    logs:
      level: "warn"
  pipelines:
    traces:
      receivers: [opencensus]
      exporters: [azuremonitor]
```
KrakenD configuration:
```json
{
    "telemetry/opencensus": {
            "sample_rate": 100,
            "reporting_period": 60,
            "exporters": {
                "ocagent": {
                  "address": "otel-collector:55678",
                  "service_name": "myKrakenD",
                  "insecure": true
                }
            }
        }
}
```
Docker-compose sample:
```yaml
version: "3"
services:
  krakend:
    image: devopsfaith/krakend:2
    command: [ "krakend", "run", "-d", "-c", "/etc/krakend/krakend.json"]
    volumes:
      - ./:/etc/krakend
    ports:
      - 8080:8080
    depends_on:
      - otel-collector
  # Collector
  otel-collector:
    image: otel/opentelemetry-collector-contrib
    command: ["--config=/etc/otel-collector.yaml"]
    environment:
      - GRPC_GO_LOG_SEVERITY_LEVEL=warn
      - GRPC_GO_LOG_VERBOSITY_LEVEL=warn
    volumes:
      - ./otel-collector.yaml:/etc/otel-collector.yaml
    ports:
      - 55678:55678
```
