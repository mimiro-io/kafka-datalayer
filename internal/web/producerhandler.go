package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/bcicen/jstream"
	"github.com/labstack/echo/v4"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/coder"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"
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
		OnStart: func(ctx context.Context) error {
			e.POST("/datasets/:dataset/entities", ph.produce, mw.authorizer(log, "datahub:w"))
			return nil
		},
	})

}

func (ph *producerHandler) produce(c echo.Context) error {
	datasetName, _ := url.QueryUnescape(c.Param("dataset"))

	// parse it
	batchSize := 10000
	read := 0

	entities := make([]*coder.Entity, 0)

	isFirst := true
	ctx := &coder.Context{}

	err := coder.ParseStream(c.Request().Body, func(value *jstream.MetaValue) error {
		if isFirst {
			ctx = coder.AsContext(value)
			isFirst = false
		} else {
			entities = append(entities, coder.AsEntity(value))
			read++
			if read == batchSize {
				read = 0

				// do stuff with entities
				err2 := ph.producers.ProduceEntities(datasetName, ctx, entities)
				if err2 != nil {
					return err2
				}
				entities = make([]*coder.Entity, 0)
			}
		}
		return nil
	})

	if err != nil {
		ph.log.Warn(err)
		return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
	}

	if read > 0 {
		// do stuff with leftover entities
		err = ph.producers.ProduceEntities(datasetName, ctx, entities)
		if err != nil {
			ph.log.Warn(err)
			return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
		}
	}

	return c.NoContent(http.StatusOK)
}
