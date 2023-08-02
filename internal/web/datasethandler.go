package web

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type DatasetName struct {
	Name string   `json:"name"`
	Type []string `json:"type"`
}

type datasetHandler struct {
	logger *zap.SugaredLogger
	mngr   *conf.ConfigurationManager
}

func NewDatasetHandler(lc fx.Lifecycle, e *echo.Echo, logger *zap.SugaredLogger, mw *Middleware, mngr *conf.ConfigurationManager) {
	log := logger.Named("web")

	dh := &datasetHandler{
		logger: log,
		mngr:   mngr,
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			e.GET("/datasets", dh.listDatasetsHandler, mw.authorizer(log, "datahub:r"))
			return nil
		},
	})
}

// listDatasetsHandler
func (handler *datasetHandler) listDatasetsHandler(c echo.Context) error {
	datasets := make([]DatasetName, 0)
	existing := make(map[string]DatasetName)

	for _, v := range handler.mngr.Datalayer.Producers {
		if _, ok := existing[v.Dataset]; !ok {
			existing[v.Dataset] = DatasetName{Name: v.Dataset, Type: []string{"POST"}}
		}
	}

	for _, v := range handler.mngr.Datalayer.Consumers {
		if d, ok := existing[v.Dataset]; ok {
			d.Type = []string{"GET", "POST"}
			existing[v.Dataset] = d
		} else {
			existing[v.Dataset] = DatasetName{Name: v.Dataset, Type: []string{"GET"}}
		}
	}

	for _, v := range existing {
		datasets = append(datasets, v)
	}

	return c.JSON(http.StatusOK, datasets)
}
