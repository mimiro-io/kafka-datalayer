package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	egdm "github.com/mimiro-io/entity-graph-data-model"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
)

type producerHandler struct {
	log       *zap.SugaredLogger
	producers *kafka.Producers
}

func NewProducerHandler(lc fx.Lifecycle, e *echo.Echo, logger *zap.SugaredLogger, mw *Middleware, producers *kafka.Producers) {
	log := logger.Named("web")

	ph := &producerHandler{
		log:       log,
		producers: producers,
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			e.POST("/datasets/:dataset/entities", ph.produce, mw.authorizer(log, "datahub:w"))
			return nil
		},
	})
}

func (ph *producerHandler) produce(c echo.Context) error {
	datasetName, _ := url.QueryUnescape(c.Param("dataset"))
	config := ph.producers.ConfigForDataset(datasetName)
	if config == nil {
		return echo.NewHTTPError(http.StatusNotFound, errors.New("dataset not found").Error())
	}

	// parse it
	batchSize := 10000
	read := 0

	entities := make([]*egdm.Entity, 0)

	parser := egdm.NewEntityParser(egdm.NewNamespaceContext())
	// if stripProps is enabled, the producers service will strip all namespace prefixes from the properties
	if !config.StripProps {
		// if it is NOT enabled, we will expand all namespace prefixes in the entity parser
		parser = parser.WithExpandURIs()
	}
	err := parser.Parse(c.Request().Body, func(entity *egdm.Entity) error {
		entities = append(entities, entity)
		read++
		if read == batchSize {
			read = 0

			// do stuff with entities
			err2 := ph.producers.ProduceEntities(config, entities)
			if err2 != nil {
				return err2
			}
			entities = make([]*egdm.Entity, 0)
		}
		return nil
	}, nil)
	if err != nil {
		ph.log.Warn(err)
		return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
	}

	if read > 0 {
		// do stuff with leftover entities
		err = ph.producers.ProduceEntities(config, entities)
		if err != nil {
			ph.log.Warn(err)
			return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
		}
	}

	return c.NoContent(http.StatusOK)
}