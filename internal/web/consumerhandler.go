package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/labstack/echo/v4"
	egdm "github.com/mimiro-io/entity-graph-data-model"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
)

type consumerHandler struct {
	logger    *zap.SugaredLogger
	consumers *kafka.Consumers
}

func NewConsumerHandler(lc fx.Lifecycle, e *echo.Echo, logger *zap.SugaredLogger, mw *Middleware, consumers *kafka.Consumers) {
	log := logger.Named("web")

	handler := &consumerHandler{
		logger:    log,
		consumers: consumers,
	}
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			e.GET("/datasets/:dataset/entities", handler.consume, mw.authorizer(log, "datahub:r"))
			e.GET("/datasets/:dataset/changes", handler.consume, mw.authorizer(log, "datahub:r"))

			return nil
		},
	})
}

func (handler *consumerHandler) consume(c echo.Context) error {
	datasetName, _ := url.QueryUnescape(c.Param("dataset"))
	limit := c.QueryParam("limit")
	var l int64 = -1
	if limit != "" {
		f, _ := strconv.ParseInt(limit, 10, 64)
		l = f
	}
	since := c.QueryParam("since")

	// check dataset exists
	if !handler.consumers.DoesDatasetExist(datasetName) {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(http.StatusOK)
	enc := json.NewEncoder(c.Response())

	_, err2 := c.Response().Write([]byte("["))
	if err2 != nil {
		return err2
	}

	// make and send context as the first object
	ctx := handler.consumers.GetContext(datasetName)

	_ = enc.Encode(ctx)

	request := kafka.DatasetRequest{
		DatasetName: datasetName,
		Since:       since,
		Limit:       l,
	}
	err := handler.consumers.ChangeSet(request, func(entity *egdm.Entity) error {
		_, err := c.Response().Write([]byte(","))
		if err != nil {
			return err
		}
		err = enc.Encode(entity)
		if err != nil {
			return err
		}
		c.Response().Flush()
		return nil
	}, func(continuation *egdm.Continuation) error {
		_, err := c.Response().Write([]byte(","))
		if err != nil {
			return err
		}
		err = enc.Encode(continuation)
		if err != nil {
			return err
		}
		c.Response().Flush()
		return nil
	})
	if err != nil {
		handler.logger.Warn(err)
	}

	_, err = c.Response().Write([]byte("]"))
	if err != nil {
		return err
	}
	c.Response().Flush()
	return nil
}

