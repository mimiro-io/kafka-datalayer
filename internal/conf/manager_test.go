package conf

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/security"
)

func TestLoadFile(t *testing.T) {
	cmgr := ConfigurationManager{
		logger: zap.NewNop().Sugar(),
	}
	resourcesTestPath := "../../resources/test"
	overridePath := os.Getenv("RESOURCES_TEST_DIR")
	if overridePath != "" {
		resourcesTestPath = overridePath
	}
	_, err := cmgr.loadFile("file://" + path.Join(resourcesTestPath, "/test-config.json"))
	if err != nil {
		t.FailNow()
	}
}

func TestLoadUrl(t *testing.T) {
	srv := serverMock()
	defer srv.Close()

	cmgr := ConfigurationManager{
		logger:         zap.NewNop().Sugar(),
		TokenProviders: security.NoOpTokenProviders(),
	}

	_, err := cmgr.loadURL(fmt.Sprintf("%s/test/config.json", srv.URL))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

func TestParse(t *testing.T) {
	cmgr := ConfigurationManager{
		logger: zap.NewNop().Sugar(),
	}
	resourcesTestPath := "../../resources/test"
	overridePath := os.Getenv("RESOURCES_TEST_DIR")
	if overridePath != "" {
		resourcesTestPath = overridePath
	}
	res, err := cmgr.loadFile("file://" + path.Join(resourcesTestPath, "/test-config.json"))
	if err != nil {
		t.FailNow()
	}

	config, err := cmgr.parse(res)
	if err != nil {
		t.FailNow()
	}

	if config.Producers[0].Topic != "json-producer" {
		t.Errorf("%s != json-producer", config.Producers[0].Topic)
	}
}

func serverMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/test/config.json", configMock)

	srv := httptest.NewServer(handler)

	return srv
}

func configMock(w http.ResponseWriter, r *http.Request) {
	cmgr := ConfigurationManager{
		logger: zap.NewNop().Sugar(),
	}
	resourcesTestPath := "../../resources/test"
	overridePath := os.Getenv("RESOURCES_TEST_DIR")
	if overridePath != "" {
		resourcesTestPath = overridePath
	}
	res, _ := cmgr.loadFile("file://" + path.Join(resourcesTestPath, "/test-config.json"))
	_, _ = w.Write(res)
}

