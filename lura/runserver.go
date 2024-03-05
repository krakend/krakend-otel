package lura

import (
	"context"
	"net/http"

	kotelconfig "github.com/krakend/krakend-otel/config"
	kotelhttpserver "github.com/krakend/krakend-otel/http/server"
	"github.com/krakend/krakend-otel/state"
	luraconfig "github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	luragin "github.com/luraproject/lura/v2/router/gin"
)

func GlobalRunServer(_ logging.Logger, srvCfg *luraconfig.ServiceConfig, stateFn state.GetterFn,
	otelCfgParser kotelconfig.ConfigParserFn, next luragin.RunServerFunc,
) luragin.RunServerFunc {
	otelCfg, err := otelCfgParser(*srvCfg)
	if otelCfg == nil {
		if err != nil && err != kotelconfig.ErrNoConfig {
			// TODO: we might want to log the error using otel at this layer
		}
		return next
	}

	// TODO: we might want to output some log info about using otel at this layer

	return func(ctx context.Context, cfg luraconfig.ServiceConfig, h http.Handler) error {
		wrappedH := kotelhttpserver.NewTrackingHandler(h, otelCfg, stateFn)
		return next(ctx, cfg, wrappedH)
	}
}
