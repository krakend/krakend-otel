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

func GlobalRunServer(_ logging.Logger, obsConfig *kotelconfig.Config, stateFn state.GetterFn, next luragin.RunServerFunc) luragin.RunServerFunc {
	return func(ctx context.Context, cfg luraconfig.ServiceConfig, h http.Handler) error {
		wrappedH := kotelhttpserver.NewTrackingHandler(h, obsConfig, stateFn)
		return next(ctx, cfg, wrappedH)
	}
}
