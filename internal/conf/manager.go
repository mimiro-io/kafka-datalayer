package conf

import (
	"crypto/md5"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bamzi/jobrunner"
	"github.com/gojektech/heimdall/v6/httpclient"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/security"
)

type ConfigurationManager struct {
	configLocation      string
	refreshInterval     string
	Datalayer           *KafkaConfig
	logger              *zap.SugaredLogger
	State               State
	TokenProviders      *security.TokenProviders
	updateListenerFuncs []func(digest [16]byte)
}

type State struct {
	Timestamp int64
	Digest    [16]byte
}

func NewConfigurationManager(env *Env, providers *security.TokenProviders) *ConfigurationManager {
	config := &ConfigurationManager{
		configLocation:  env.ConfigLocation,
		refreshInterval: env.RefreshInterval,
		Datalayer:       &KafkaConfig{},
		TokenProviders:  providers,
		logger:          env.Logger.Named("configuration"),
		State: State{
			Timestamp: time.Now().Unix(),
		},
	}
	config.Datalayer = config.Init()
	/*lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			config.Datalayer = config.Init()
			config.logger.Info(config.Datalayer.Consumers)
			return nil
		},
	})*/

	return config
}

func (conf *ConfigurationManager) Init() *KafkaConfig {
	conf.logger.Infof("Starting the ConfigurationManager with refresh %s\n", conf.refreshInterval)
	config := conf.load()
	conf.logger.Info("Done loading the config")
	jobrunner.Start()
	err := jobrunner.Schedule(conf.refreshInterval, conf)
	if err != nil {
		conf.logger.Warn("Could not start configuration reload job")
	}
	return config
}

func (conf *ConfigurationManager) Run() {
	conf.load()
}

func (conf *ConfigurationManager) load() *KafkaConfig {
	var configContent []byte
	var err error
	if strings.Index(conf.configLocation, "file://") == 0 {
		configContent, err = conf.loadFile(conf.configLocation)
	} else if strings.Index(conf.configLocation, "http") == 0 {
		c, err2 := conf.loadURL(conf.configLocation)
		if err2 != nil {
			conf.logger.Warn("Unable to parse json into config. Error is: "+
				err2.Error()+". Please check file: "+conf.configLocation, err2)
			return nil
		}
		configContent, err2 = unpackContent(c)
		if err2 != nil {
			conf.logger.Warn("Unable to parse json into config. Error is: "+
				err2.Error()+". Please check file: "+conf.configLocation, err2)
			return nil
		}
	} else {
		conf.logger.Errorf("Config file location not supported: %s \n", conf.configLocation)
		configContent, _ = conf.loadFile("file://resources/default-config.json")
	}
	if err != nil {
		// means no file found
		conf.logger.Infof("Could not find %s", conf.configLocation)
	}

	if configContent == nil {
		// again means not found or no content
		conf.logger.Infof("No values read for %s", conf.configLocation)
		configContent = make([]byte, 0)
	}

	state := State{
		Timestamp: time.Now().Unix(),
		Digest:    md5.Sum(configContent),
	}

	if state.Digest != conf.State.Digest {
		config, err := conf.parse(configContent)
		if err != nil {
			conf.logger.Warn("Unable to parse json into config. Error is: "+err.Error()+". Please check file: "+conf.configLocation, err)
			return nil
		}

		conf.Datalayer = config
		conf.State = state
		conf.logger.Info("Updated configuration with new values")

		for _, f := range conf.updateListenerFuncs {
			go f(state.Digest)
		}
	}
	return conf.Datalayer
}

func (conf *ConfigurationManager) loadURL(configEndpoint string) ([]byte, error) {
	timeout := 10000 * time.Millisecond
	client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout), httpclient.WithRetryCount(3))

	req, err := http.NewRequest("GET", configEndpoint, nil) //
	if err != nil {
		return nil, err
	}

	provider, ok := conf.TokenProviders.Providers["auth0tokenprovider"]
	if ok {
		tokenProvider := provider.(security.TokenProvider)
		bearer, err2 := tokenProvider.Token()
		if err2 != nil {
			conf.logger.Warnf("Token provider returned error: %w", err2)
		}
		req.Header.Add("Authorization", bearer)
	}

	resp, err := client.Do(req)
	if err != nil {
		conf.logger.Error("Unable to open config url: "+configEndpoint, err)
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == 200 {
		return io.ReadAll(resp.Body)
	} else {
		conf.logger.Infof("Endpoint returned %s", resp.Status)
		return nil, err
	}
}

type content struct {
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data"`
}

func unpackContent(themBytes []byte) ([]byte, error) {
	unpacked := &content{}
	err := json.Unmarshal(themBytes, unpacked)
	if err != nil {
		return nil, err
	}
	data := unpacked.Data

	return json.Marshal(data)
}

func (conf *ConfigurationManager) loadFile(location string) ([]byte, error) {
	configFileName := strings.ReplaceAll(location, "file://", "")

	configFile, err := os.Open(configFileName)
	if err != nil {
		conf.logger.Error("Unable to open config file: "+configFileName, err)
		return nil, err
	}
	defer configFile.Close()
	return io.ReadAll(configFile)
}

func (conf *ConfigurationManager) parse(config []byte) (*KafkaConfig, error) {
	configuration := &KafkaConfig{}
	err := json.Unmarshal(config, configuration)
	return configuration, err
}

func (conf *ConfigurationManager) AddConfigUpdateListener(update func(digest [16]byte)) {
	conf.updateListenerFuncs = append(conf.updateListenerFuncs, update)
}