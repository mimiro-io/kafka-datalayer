package web

import (
	"context"
	"net/http"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
)

type Server struct {
	logger       *zap.SugaredLogger
	statsdClient statsd.ClientInterface
	e            *echo.Echo
	kafka        *kafka.Service
}

func NewWebServer(env *conf.Env,
	k *kafka.Service,
	logger *zap.SugaredLogger,
	statsd statsd.ClientInterface) (*Server, error) {
	port := env.Port
	e := echo.New()
	e.HideBanner = true

	mw := NewMiddleware(logger, statsd, env)
	e.Use(mw...)

	l := logger.Named("web")
	s := &Server{
		logger:       l,
		statsdClient: statsd,
		e:            e,
		kafka:        k,
	}
	e.GET("/health", health)
	e.POST("/datasets/:dataset/entities", s.produce)
	e.GET("/datasets/:dataset/entities", s.consume)
	e.GET("/datasets/:dataset/changes", s.consume)
	e.GET("/datasets", s.listDatasetsHandler)

	l.Infof("Starting Http server on :%s", port)
	go func() {
		_ = e.Start(":" + port)
	}()

	return s, nil
}

func (w *Server) Shutdown(ctx context.Context) error {
	return w.e.Shutdown(ctx)
}

func health(c echo.Context) error {
	return c.String(http.StatusOK, "UP")
}