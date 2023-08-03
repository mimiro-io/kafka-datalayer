package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type DatasetName struct {
	Name string   `json:"name"`
	Type []string `json:"type"`
}

// listDatasetsHandler
func (w *Server) listDatasetsHandler(c echo.Context) error {
	datasets := make([]DatasetName, 0)
	existing := make(map[string]DatasetName)

	for _, v := range w.kafka.Mngr.Datalayer.Producers {
		if _, ok := existing[v.Dataset]; !ok {
			existing[v.Dataset] = DatasetName{Name: v.Dataset, Type: []string{"POST"}}
		}
	}

	for _, v := range w.kafka.Mngr.Datalayer.Consumers {
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