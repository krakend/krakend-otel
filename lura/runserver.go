package lura

import (
	"context"
	"net/http"

	kotelhttpserver "github.com/krakend/krakend-otel/http/server"
	"github.com/krakend/krakend-otel/state"
	luraconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	luragin "github.com/luraproject/lura/v2/router/gin"
)

func GlobalRunServer(_ logging.Logger, next luragin.RunServerFunc) luragin.RunServerFunc {
	otelCfg := state.GlobalConfig()
	if otelCfg == nil {
		return next
	}

	return func(ctx context.Context, cfg luraconfig.ServiceConfig, h http.Handler) error {
		wrappedH := kotelhttpserver.NewTrackingHandler(h)
		return next(ctx, cfg, wrappedH)
	}
}
