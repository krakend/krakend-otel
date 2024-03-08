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
	otellura "github.com/krakend/krakend-otel/lura"
	otelgin "github.com/krakend/krakend-otel/router/gin"
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
		fmt.Printf("ERROR: %s\n", err.Error())
		cancel()
		return
	}
	serviceConfig.Debug = serviceConfig.Debug || *debug
	if *port != 0 {
		serviceConfig.Port = *port
	}

	logger, _ := logging.NewLogger(*logLevel, os.Stdout, "[KRAKEND]")

	shutdownFn, err := kotel.Register(ctx, logger, serviceConfig)
	if err != nil {
		fmt.Printf("--- failed to register: %s\n", err.Error())
		cancel()
		return
	}
	defer shutdownFn()

	bf := func(backendConfig *config.Backend) proxy.Proxy {
		reqExec := otellura.HTTPRequestExecutorFromConfig(client.NewHTTPClient,
			backendConfig)
		return proxy.NewHTTPProxyWithHTTPExecutor(backendConfig, reqExec, backendConfig.Decoder)
	}
	bf = otellura.BackendFactory(bf)

	defaultPF := proxy.NewDefaultFactory(bf, logger)
	pf := otellura.ProxyFactory(defaultPF)

	handlerF := otelgin.New(krakendgin.EndpointHandler)

	runserverChain := krakendgin.RunServerFunc(
		otellura.GlobalRunServer(logger, server.RunServer))

	engine := gin.Default()
	engine.RedirectTrailingSlash = true
	engine.RedirectFixedPath = true
	engine.HandleMethodNotAllowed = true
	engine.ContextWithFallback = true // <- this is important for trace span propagation

	// setup the krakend router
	routerFactory := krakendgin.NewFactory(krakendgin.Config{
		Engine:         engine,
		ProxyFactory:   pf,
		Middlewares:    []gin.HandlerFunc{},
		Logger:         logger,
		HandlerFactory: handlerF,
		RunServer:      runserverChain,
	})

	// start the engine
	routerFactory.NewWithContext(ctx).Run(serviceConfig)
}
