# Implementation Details

How the code is organized:

- [io package](../io): has the wrappers for basic io classes to instrument the 
  bytes read / written with an `io.Reader` / `io.Writer` (event the
  writer is not currently used in other packages)
  
- [http package](../http): has the common functions used to instrument an http
  request or response (currently, it only has a way to extract
  trace attributes).

- [http/client package](../http/client): this package provides the wrapper
  for an instrumented http client. It has all the 
  
- [config package](../config): this package contains the configuration
  definitions for all the krakend-otel library
  
- [state package](../state): contains exporter instances
  and the meter and tracer instances created from a configuration
  (also a global shared state that can be used from anywhere).
    
- [exporter package](../exporter): here are the interfaces to implement
  the metric / traces exporters, as well as specific implementations
  for some "providers", like prometheus, otel collector, ...
  
- [lura package](../lura): specific Lura middlewares to instruments
  the `pipe` and `backend` stages.
  
- [router/gin](../router/gin): contains specifig gin middleware to
  instrument a gin router.
  
- [example](../example): an example of how to use the krakend-otel 
  library: check the [example documentation](../example/README.md) for
  more info.
