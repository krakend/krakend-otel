# Start the environment


# Grafana

Go to `http://localhost:3000` and login with `krakend` :: `krakend`.


# Generate traces

In the [opentelemetry.io collector getting started page](https://opentelemetry.io/docs/collector/getting-started/)'s 
step `2` we can see that we can download a tool to generate traces:

```
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest
```

The `gRPC` endpoint is on port `4317` by default.

```
telemetrygen traces --otlp-insecure --duration 5s
```


## References

- [Grafana's Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [Grafana's tempo docker-compose example](https://github.com/grafana/tempo/tree/main/example/docker-compose)
