/*
Srv creates a few basic routes to test the configured instrumentation.

Usage:

	srv [flags]

The flags are:

	-p [port_number]
	    To select the port number where we want to run the server

	-d
	    To enable debug logs

	-c [config_file]
	    To select the config file to use.
*/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/transport/http/client"
	"github.com/luraproject/lura/v2/transport/http/server"

	kotel "github.com/krakend/krakend-otel"
	// "github.com/krakend/krakend-otel/exporter"
	kotelconfig "github.com/krakend/krakend-otel/config"
	"github.com/krakend/krakend-otel/lura"
	otelgin "github.com/krakend/krakend-otel/router/gin"
	"github.com/krakend/krakend-otel/state"
)

func main() {
	port := flag.Int("p", 0, "Port of the service")
	logLevel := flag.String("l", "ERROR", "Logging level")
	debug := flag.Bool("d", false, "Enable the debug")
	configFile := flag.String("c", "/etc/krakend/configuration.json", "Path to the configuration filename")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case sig := <-sigs:
			log.Println("Signal intercepted:", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	parser := config.NewParser()
	serviceConfig, err := parser.Parse(*configFile)
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}
	serviceConfig.Debug = serviceConfig.Debug || *debug
	if *port != 0 {
		serviceConfig.Port = *port
	}

	// TODO: how to register the exporter factory
	logger, _ := logging.NewLogger(*logLevel, os.Stdout, "[KRAKEND]")

	obsConfig, err := kotelconfig.FromLura(serviceConfig)

	if err != nil {
		fmt.Printf("ERROR: no config found for open telemetry: %s\n", err.Error())
		return
	}

	if err := kotel.Register(ctx, serviceConfig); err != nil {
		log.Fatal(err)
		fmt.Printf("--- failed to register\n")
		return
	}

	bf := func(backendConfig *config.Backend) proxy.Proxy {
		reqExec := lura.HTTPRequestExecutorFromConfig(client.NewHTTPClient,
			backendConfig, obsConfig.Layers.Backend)
		return proxy.NewHTTPProxyWithHTTPExecutor(backendConfig, reqExec, backendConfig.Decoder)
	}

	defaultPF := proxy.NewDefaultFactory(lura.BackendFactory(bf, state.GlobalState, obsConfig.Layers.Backend), logger)
	pf := lura.ProxyFactory(defaultPF, state.GlobalState, obsConfig.Layers.Pipe)

	// setup the krakend router
	routerFactory := krakendgin.NewFactory(krakendgin.Config{
		Engine:         gin.Default(),
		ProxyFactory:   pf,
		Middlewares:    []gin.HandlerFunc{},
		Logger:         logger,
		HandlerFactory: otelgin.New(krakendgin.EndpointHandler, state.GlobalState, obsConfig.Layers.Router),
		RunServer:      server.RunServer,
	})

	// start the engine
	routerFactory.NewWithContext(ctx).Run(serviceConfig)
}
