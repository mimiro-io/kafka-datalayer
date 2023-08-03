package kafka

import (
	"context"

	"github.com/DataDog/datadog-go/v5/statsd"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

type Service struct {
	Producers *Producers
	Consumers *Consumers
	Mngr      *conf.ConfigurationManager
}

func (s Service) Shutdown(_ context.Context) error {
	s.Consumers.adminClient.Close()
	s.Producers.adminClient.Close()
	return nil
}

func NewService(conf *conf.Env, logger *zap.SugaredLogger, mngr *conf.ConfigurationManager, statsdClient statsd.ClientInterface) (*Service, error) {
	p, err := NewProducers(conf, logger, mngr, statsdClient)
	if err != nil {
		return nil, err
	}
	c, err := NewConsumers(conf, mngr, logger, statsdClient)
	if err != nil {
		return nil, err
	}
	return &Service{
		Producers: p,
		Consumers: c,
		Mngr:      mngr,
	}, nil
}