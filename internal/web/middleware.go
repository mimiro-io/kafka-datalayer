package web

import (
	"strings"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/web/middlewares"
)

type Middleware []echo.MiddlewareFunc

func NewMiddleware(logger *zap.SugaredLogger, statsdClient statsd.ClientInterface, env *conf.Env) Middleware {
	skipper := func(c echo.Context) bool {
		// don't secure health endpoints
		return strings.HasPrefix(c.Request().URL.Path, "/health")
	}

	m := Middleware{setupLogger(logger, statsdClient, skipper)}

	if env.Auth.Middleware == "noop" { // don't enable local security (yet)
		logger.Infof("WARNING: Security is disabled")
	} else {
		m = append(m, setupCors())
		m = append(m, setupJWT(env.Auth, skipper))
	}
	m = append(m, setupRecovery(logger))
	return m
}

func setupJWT(conf *conf.AuthConfig, skipper func(c echo.Context) bool) echo.MiddlewareFunc {
	return middlewares.JWTHandler(&middlewares.Auth0Config{
		Skipper:       skipper,
		Audience:      conf.Audience,
		Issuer:        conf.Issuer,
		AudienceAuth0: conf.AudienceAuth0,
		IssuerAuth0:   conf.IssuerAuth0,
		Wellknown:     conf.WellKnown,
	})
}

func setupLogger(logger *zap.SugaredLogger, statsdClient statsd.ClientInterface,
	skipper func(c echo.Context) bool) echo.MiddlewareFunc {
	return middlewares.LoggerFilter(middlewares.LoggerConfig{
		Skipper:      skipper,
		Logger:       logger.Desugar(),
		StatsdClient: statsdClient,
	})
}

func setupCors() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"https://api.mimiro.io", "https://platform.mimiro.io"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	})
}

func setupRecovery(logger *zap.SugaredLogger) echo.MiddlewareFunc {
	return middlewares.RecoverWithConfig(middlewares.DefaultRecoverConfig, logger)
}