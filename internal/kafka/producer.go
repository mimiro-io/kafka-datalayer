package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/hashicorp/go-uuid"
	egdm "github.com/mimiro-io/entity-graph-data-model"
	kgo "github.com/segmentio/kafka-go"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

type Producers struct {
	log              *zap.SugaredLogger
	env              *conf.Env
	bootstrapServers []string
	adminClient      *kafka.AdminClient
	producers        map[string]*kgo.Writer
	mngr             *conf.ConfigurationManager
	statsd           statsd.ClientInterface
}

func NewProducers(lc fx.Lifecycle, env *conf.Env, mngr *conf.ConfigurationManager, statsd statsd.ClientInterface) (*Producers, error) {
	producers := &Producers{
		log:              env.Logger.Named("producers"),
		env:              env,
		bootstrapServers: env.KafkaBrokers,
		producers:        make(map[string]*kgo.Writer),
		mngr:             mngr,
		statsd:           statsd,
	}

	onUpdate := func(_ [16]byte) {
		err := producers.initTopics(mngr.Datalayer.Producers)
		if err != nil {
			producers.log.Warn(err)
		}
	}

	a, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": strings.Join(producers.bootstrapServers, ",")})
	if err != nil {
		return nil, err
	}
	producers.adminClient = a

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			mngr.AddConfigUpdateListener(onUpdate)
			if mngr.Datalayer.Producers != nil && len(mngr.Datalayer.Producers) > 0 {
				return producers.initTopics(mngr.Datalayer.Producers)
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			producers.log.Info("Stopping admin client")
			a.Close()
			return nil
		},
	})

	return producers, nil
}

func (producers *Producers) DoesDatasetExist(datasetName string) bool {
	if producers.mngr.Datalayer.Producers == nil {
		return false
	}

	for _, c := range producers.mngr.Datalayer.Producers {
		if c.Dataset == datasetName {
			return true
		}
	}
	return false
}

func stripProps(entity *egdm.Entity) ([]byte, error) {
	stripped := make(map[string]interface{})
	stripped["id"] = strings.SplitAfter(entity.ID, ":")[1]
	stripped["deleted"] = entity.IsDeleted

	singleMap := make(map[string]interface{})
	for e := range entity.Properties {
		singleMap[strings.SplitAfter(e, ":")[1]] = entity.Properties[e]
	}
	stripped["props"] = singleMap
	return json.Marshal(stripped)
}

func (producers *Producers) ProduceEntities(datasetName string, entities []*egdm.Entity) error {
	config := producers.configForDataset(datasetName)

	var w *kgo.Writer
	if prod, ok := producers.producers[datasetName]; !ok {
		w = &kgo.Writer{
			Addr:     kgo.TCP(producers.bootstrapServers...),
			Topic:    config.Topic,
			Balancer: kgo.Murmur2Balancer{},
		}
		producers.producers[datasetName] = w
	} else {
		w = prod
	}

	tags := []string{
		fmt.Sprintf("application:%s", producers.env.ServiceName),
		fmt.Sprintf("topic:%s", config.Topic),
	}

	data := make([]kgo.Message, len(entities))
	for i, entity := range entities {
		var themBytes []byte
		if config.StripProps {
			raw, err := stripProps(entity)
			if err != nil {
				return err
			}
			themBytes = raw
		} else {
			raw, err := json.Marshal(entity)
			if err != nil {
				return err
			}
			themBytes = raw
		}
		data[i] = kgo.Message{
			Key:   producers.determineKey(entity, config),
			Value: themBytes,
		}
		_ = producers.statsd.Incr("kafka.write", tags, 1)
	}
	return w.WriteMessages(context.Background(), data...)
}

func (producers *Producers) determineKey(entity *egdm.Entity, config *conf.ProducerConfig) []byte {
	if config.Key == nil {
		return nil
	}
	switch *config.Key {
	case "id":
		return []byte(entity.ID)
	case "uuid":
		id, _ := uuid.GenerateUUID()
		return []byte(id)
	default:
		return nil
	}
}

func (producers *Producers) configForDataset(datasetName string) *conf.ProducerConfig {
	for _, c := range producers.mngr.Datalayer.Producers {
		if c.Dataset == datasetName {
			return &c
		}
	}
	return nil
}

func (producers *Producers) initTopics(config []conf.ProducerConfig) error {
	specs := make([]kafka.TopicSpecification, 0)
	for _, c := range config {
		producers.log.Info("Verifying topic -> " + c.Topic)
		if c.CreateTopic {
			producers.log.Info("Creating topic if not already existing")
			topicConfig := make(map[string]string)
			if c.TopicSettings.Config != nil {
				for k, v := range *c.TopicSettings.Config {
					topicConfig[k] = v
				}
			}
			if _, ok := topicConfig["retention.ms"]; !ok {
				topicConfig["retention.ms"] = "-1" // we set it to live forever if it is not set
			}

			specs = append(specs, kafka.TopicSpecification{
				Topic:             c.Topic,
				NumPartitions:     c.TopicSettings.Partitions,
				ReplicationFactor: c.TopicSettings.Replicas,
				Config:            topicConfig,
			})
		}
	}

	if len(specs) > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		maxDur := 60 * time.Second

		results, err := producers.adminClient.CreateTopics(ctx, specs, kafka.SetAdminOperationTimeout(maxDur))
		if err != nil {
			return err
		}
		for _, result := range results {
			producers.log.Infof("%s\n", result)
		}
	}
	return nil
}
