package web

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	egdm "github.com/mimiro-io/entity-graph-data-model"
)

func (w *Server) produce(c echo.Context) error {
	datasetName, _ := url.QueryUnescape(c.Param("dataset"))
	config := w.kafka.Producers.ConfigForDataset(datasetName)
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
			err2 := w.kafka.Producers.ProduceEntities(config, entities)
			if err2 != nil {
				return err2
			}
			entities = make([]*egdm.Entity, 0)
		}
		return nil
	}, nil)
	if err != nil {
		w.logger.Warn(err)
		return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
	}

	if read > 0 {
		// do stuff with leftover entities
		err = w.kafka.Producers.ProduceEntities(config, entities)
		if err != nil {
			w.logger.Warn(err)
			return echo.NewHTTPError(http.StatusBadRequest, errors.New("could not parse the json payload").Error())
		}
	}

	return c.NoContent(http.StatusOK)
}