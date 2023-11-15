# krakend-otel

OpenTelemetry building blocks to instrument [Lura](https://github.com/luraproject/lura) / [KrakenD](https://krakend.io) 
based API Gateways.

[Apache Licnese 2.0](.LICENSE)

[Check the official KrakenD Open Telemetry documentation](https://www.krakend.io/docs/telemetry/opentelemetry/)


## Example

For a quick look at the observability the library can provide, check the 
[example documentation](./example/README.md).

## Configuration from Lura's [ServiceConfig](https://github.com/luraproject/lura/blob/master/config/config.go)

In order to configure the open telemetry stack to instrument the API Gateway, a 
new entry must be added to the `ExtraConfig` root propertry of `ServiceConfig` 
using the `telemetry/opentelemetry` key, with the `krakend-otel`'s configuration.

## `krakend-otel` Configuration

In the configuration we find the following root entries:

- `service_name`: to provide a custom name for this service. However if there is 
  a `ServiceConfig.Name` already set, that will be the one used.
- `[exporters](exporters)`: in this section we define our different exporter
  configurations, giving them our own "custom name" to be referenced later
  when we deine the instance. This allows us to use the same `kind` of exporter,
  with different port / host configurations, and report the same metric / traces
  to different systems (imagine migrating from a self hosted grafana stack to the
  cloud version, and that for a while you want to report to both places before
  making the switch), of report to different `kind` of exporters (migrating from one cloud 
  observability system to a different one).
- `layers`: in this section we can fine tune the amount of metrics / traces that 
  we want to report at each "stage" of the processing pipeline: `router`, `pipe` or `backend`.
- `skip_paths`: an option to define endpoints that we do not want to instrument. By default,
    if it is not provided, it will skip the following "internal" paths:
		- `/healthz`
		- `/_ah/health`
		- `/__debug`
		- `/__echo`
    If we want to instrument those paths too, just provide an empty string as endpoint to
    skip.

In a visual way, this is the realation between the `exporters` configuration, and 
how we select as `metric_providers` or `trace_providers`: 

![krakend_otel_exporters.svg](./doc/krakend_otel_exporters.svg)


However, for providers that can export both metric and traces (like in the OTLP case), you
can disable any of those.

### Exporters 

Example:

```json
"exporters": [
    {
        "name": "local_prometheus",
        "kind": "prometheus",
        "config": {
            "port": 9092,
            "process_metrics": true,
            "go_metrics": true
        }
    },
    {
        "name": "local_tempo",
        "kind": "opentelemetry",
        "config": {
            "port": 4317,
            "use_http": false
        }
    },
    { 
        "name": "local_jaeger",
        "kind": "opentelemetry",
        "config": {
            "port": 5317,
            "use_http": false
        }
    }
]
```

### Layers

We can differentiate the processing of a request in KrakenD in 3 main stages (each one
including or wrapping the inner stage):

- `global`: this part that comes before the `Lura`'s framework starts working with
    the request. In the case of [KrakenD CE](https://github.com/krakend/krakend-ce),
    this stage is implemented usin [gin](https://github.com/gin-gonic/gin)

- `proxy`: this is the `Lura`'s framework part where it deals with one of the
    API Gateway exposed endpoints, and includes spawning the required 
    requests to the backends, as well as the manipulation at the endpoint
    level before and after the requests are performed.
    
- `backend`: this is the `Lura`'s framework part where it deals with each
    single backend request (including the manipulation at that request level).
    
    
For each of those layers it can be selected the deatail of metrics and traces
that we want to report.

#### global

At the router level we have 3 main options:

- `disable_metrics`: boolean to enable / disable if we want to report metrics for this layer
- `disable_traces`: boolean to enable / disable if we want to report traces for this layer
- `disable_propagation`: boolean to disable the consumption of a propagation header for
    traces (so spans from a previous layer are linked to the KrakenD trace).

```json
"global": {
    "disable_metrics": false,
    "disable_traces": false,
    "disable_propagation": false
}
```

##### Metrics

- `http.server.duration`: histogram of the time it takes to produce the response.
    Attributes:
    - `http.request.method`: the HTTP method (`GET`, `POST`, `HEAD`, ...)
    - `http.response.status_code`: status code of the produced response
    - `url.path`: the matched endpoint path
- `http.server.response.size`: histogram of the size of the body produced for the response.
    Attributes:
    - `http.request.method`: the HTTP method (`GET`, `POST`, `HEAD`, ...)
    - `http.response.status_code`: status code of the produced response
    - `url.path`: the matched endpoint path


##### Traces

A trace is created with the received request's **path** (not the matched endpoint, because
the trace is started before any matching is performed). 

The attributes of the global trace are:

- `krakend.stage`: always with `global` value (to easily find all global traces)
- `url.path`: the matched endpoint (in case no matching is made, for example in
    `404` results, it would be empty)
- `http.response.status_code`: the response status code
- `http.response.body.size`: the returned body size

In case an error happens, the span will record the error and set the status to 
error (value = `1`), in case of success the status is set to ok (value = `2`).

#### pipe

At the pipe level we only have 2 options:

- `metrics`: boolean to enable / disable if we want to report metrics for this layer
- `traces`: boolean to enable / disable if we want to report traces for this layer

```json
"proxy": {
    "disable_metrics": false,
    "disable_traces": true
}
```

##### Metrics

- `krakend.pipe.duration`: histogram of the time it takes to produce the response.
    Attributes:
    - `url.path`: the matched endpoint path that **krakend is serving** (is different
        than in `backend`, krakend stage, when this property is the path
        for the backend we are targetting).

Attributes:

- `krakend.stage`: always with value `pipe`
- `complete`: a `true` / `false` value to know when a response is complete (all
    backends returned a successful response).
- `canceled`: if appears, will always be `true`, and indicates a request
    that has been canceled (usually when parallel requests are used).
- `error`: in case an error happened, the description of the error.

#### backend

At the backend level is where we have more granularity selecting the information
that we want to obtain.

There are three entries:

- `metrics`: to define the amount of info we want to report in backend metrics
- `traces`: to define the amount of info we want to report in backend traces
  
For both, the `metrics` and `traces` part, we can select the same options:

- `disable_stage`: to enable metrics / traces for the full backend processing part
- `round_trip`: to enable metrics /traces for the actual http request for the backend
  (not taking into account the manipulation part at the backend level).
- `read_payload`: to enable metrics / traces only for the response reading payload
  (not taking into account the http connection part of the request).
- `detailed_connection`: to enable metrics / traces for the connection details, like
  time to query the DNS, the time spent in TLS, and so one.
- `static_attributes`: an array of `key: value` pairs to be used as tags / labels in 
  the reported metric / traces.
  
```json
"backend": {
    "metrics": {
        "disable_stage": false,
        "round_trip": true,
        "read_payload": true,
        "detailed_connection": true,
        "static_attributes": [
            {
                "key": "my_metric_attr",
                "value": "my_middle_metric"
            }
        ]
    },
    "traces": {
        "disable_stage": false,
        "round_trip": true,
        "read_payload": true,
        "detailed_connection": true,
        "static_attributes": [
            {
                "key": "my_trace_attr",
                "value": "my_middle_trace" 
            }
        ]
    }
}
```

##### Metrics

- `krakend.backend.duration`: histogram of the time it takes to produce the response. Controlled
  by the `disable_stage` flag (if set to `true` this metric will not appear).
    Attributes:
    - `http.request.method`: the method used to make the request
    - `url.path`: the matched endpoint path that **krakend is serving** (is different
        than in `backend`, krakend stage, when this property is the path
    - `krakend.endpoint`: this attribute is set to the krakend exposed endpoint that 
        is the "parent" of the backend request.
    - `server.address`: the target host (in case more than one are provided, those
        are joined with `_`).

###### RoundTrip metrics

The following metrics are enabled if `round_trip` is set to true, and share the same attributes:
    - `http.request.method_original`: the methos used for the request to the backend 
      (GET, POST,...)
    - `url.path`: the requested path
    - `server.address`: the target host (just the first in the list of provided hosts).
        (**TODO**: check if we want the concat list of hosts like in the `stage-duration`
        metric, or if we should change that attribute there).
    - `krakend.stage`: always with value `backend-request`

- `http.client.duration`: histogram with the time taken since starting a request, until
    until having the first byte of the body ready to read.

- `http.client.request.started.count`: number of requests started.
- `http.client.request.failed.count`: number of requests [failed](failed).
- `http.client.request.canceled.count`: number of canceled request.
- `http.client.request.timedout.count`: number of timed out requests.
- `http.client.request.size`: counter with the sum of `Content-Length` header for the 
    sent payload for the request.

- `http.client.response.size`: histogram with the size of response bodies 
    as read from the `Content-Length` header.


###### Read Payload metrics

- `read-size`: counted with the read bytes (**TODO** should we remove this onw as is redundant with
    the histogram).
- `read-size-hist`: histogram with the read bytes
- `read-time`: counter with seconds spent reading the body payload of the response.(**TODO** remove
    if this is redundant wiht the `read-time-hist`).
- `read-time-hist`: histogram with the seconds spent reading the bdy.
- `read-errors`: counter of number of errors that happened reading the response body.
   
###### Detailed connection metrics

The following metrics are enabled if `detailed_connection` is set to **_true_**, and share the same attributes:
    - `http.request.method_original`: the methos used for the request to the backend 
      (GET, POST,...)
    - `url.path`: the requested path
    - `server.address`: the target host (just the first in the list of provided hosts).
        (**TODO**: check if we want the concat list of hosts like in the `stage-duration`
        metric, or if we should change that attribute there).
    - `krakend.stage`: always with value `backend-request`

- `request-get-conn-latency`: time to get a connection from the connection pool
- `request-dns-latency`: time spen5 resolving a DNS name
- `request-tls-latency`: time spent on TLS Handshake

##### Traces

**Stage Span** attributes (controlled by the `disable_stage` flag: will not appear if set to `true`):

- `krakend.stage`: always with value `backend`
- `complete`: a `true` / `false` value to know when a response is complete (all
    backends returned a successful response).
- `canceled`: if appears, will always be `true`, and indicates a request
    that has been canceled (usually when parallel requests are used).
- `error`: in case an error happened, the description of the error.

**read-tracer** span with when `read_paylaod` option is set to true.

### Instance 

Given that any Exporter could implement both traces and metrics, we might want
to select to use it only for one of those roles: that's why we have this section
to specify what to use. Also, it allows to easily enable / disable exporters
without needing to delete the exporters configuration.

The fields of the `instance` section are:

- `metric_providers`: this is the list of the exporter names that we want to use to 
  report metrics. Provides an easy way to just select a few of the configured exporters
  for metrics (the exporters might have support for metrics **and / or** traces, so
  that should be taken into account because if we try to use an exporter that 
  does not have support for metrics, it will fail).
- `metric_reporting_period`: how often we want to flush the metrics in seconds.
- `trace_providers`: the list of the exporter names that we want to use to
  report traces. Provides an easy way to just select a few of the configured exporters
  for traces (the exporters might have support for metrics **and / or** traces, so
  that should be taken into account because if we try to use an exporter that 
  does not have support for traces, it will fail).
- `trace_sample_rate`: a number between `0` and `1.0` to define the sample rate for 
  traces: if we set it for example to `0.25` only one in four requests will be 
  reporting traces (usefull to reduce the amount of data generated while keeping
  some traces to identify / debug issues).

Example:

```json
"instance": {
    "metric_providers": [
        "local_prometheus"
    ],
    "metric_reporting_period": 1,
    "trace_providers": [
        "local_tempo",
        "local_jaeger"
    ],
    "trace_sample_rate": 1
}
```

### Example configuration:

Putting it all together, here we have an example of a configuration:

```json
"telemetry/opentelemetry": {
    "service_name": "krakend_middle_service",
    "exporters": [
        {
            "name": "local_prometheus",
            "kind": "prometheus",
            "config": {
                "port": 9092,
                "process_metrics": true,
                "go_metrics": true
            }
        },
        {
            "name": "local_tempo",
            "kind": "opentelemetry",
            "config": {
                "port": 4317,
                "use_http": false
            }
        },
        {
            "name": "local_jaeger",
            "kind": "opentelemetry",
            "config": {
                "port": 5317,
                "use_http": false
            }
        }
    ],
    "layers": {
        "global": {
            "disable_metrics": false,
            "disable_traces": false,
            "disable_propagation": false
        },
        "proxy": {
            "disable_metrics": false,
            "disable_traces": false
        }, 
        "backend": {
            "metrics": {
                "disable_stage": false,
                "round_trip": true,
                "read_payload": true,
                "detailed_connection": true,
                "static_attributes": {
                    "my_metric_attr": "my_middle_metric"
                }
            },
            "traces": {
                "disable_stage": false,
                "round_trip": true,
                "read_payload": true,
                "detailed_connection": true,
                "static_attributes": {
                    "my_trace_attr": "my_middle_trace" 
                }
            }
        }
    },
    "instance": {
        "metric_providers": [
            "local_prometheus"
        ],
        "metric_reporting_period": 1,
        "trace_providers": [
            "local_tempo",
            "local_jaeger"
        ],
        "trace_sample_rate": 1
    },
    "skip_paths": [""],
    "extra": {
        "custom": "extra",
        "future_expansion": true
    }
}
```

# Other Info

- Check the [example documentation](./example/README.md)
- Some [notes about current implementation](./doc/implementation_details.md)
