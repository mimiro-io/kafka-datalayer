package conf

import (
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/spf13/viper"
)

func NewStatsd(env *Env) (statsd.ClientInterface, error) {
	var client statsd.ClientInterface
	agentEndpoint := viper.GetViper().GetString("DD_AGENT_HOST")
	service := viper.GetViper().GetString("SERVICE_NAME")
	if agentEndpoint != "" {
		optNs := statsd.WithNamespace(service)
		optTags := statsd.WithTags([]string{
			"service:" + env.ServiceName,
			"application:" + env.ServiceName,
		})
		env.Logger.Info("Statsd is configured on: ", viper.GetViper().GetString("DD_AGENT_HOST"))
		c, err := statsd.New(viper.GetViper().GetString("DD_AGENT_HOST"), optNs, optTags)
		if err != nil {
			return nil, err
		}
		client = c
	} else {
		env.Logger.Debug("Using NoOp statsd client")
		client = &statsd.NoOpClient{}
	}

	return client, nil
}