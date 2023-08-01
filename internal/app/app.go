package app

import (
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"

	"go.uber.org/fx"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/security"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/web"
)

func Wire() *fx.App {
	return fx.New(
		fx.Provide(
			conf.NewEnv,
			conf.NewLogger,
			conf.NewStatsd,
			security.NewTokenProviders,
			conf.NewConfigurationManager,
			web.NewWebServer,
			web.NewMiddleware,
			kafka.NewProducers,
			kafka.NewConsumers,
		),
		fx.Invoke(
			web.Register,
			web.NewDatasetHandler,
			web.NewProducerHandler,
			web.NewConsumerHandler,
		),
	)
}
