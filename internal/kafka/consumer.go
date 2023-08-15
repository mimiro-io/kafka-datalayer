package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/coder"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
)

type Consumers struct {
	env              *conf.Env
	logger           *zap.SugaredLogger
	adminClient      *kafka.AdminClient
	bootstrapServers []string
	mngr             *conf.ConfigurationManager
	statsd           statsd.ClientInterface
	running          map[string]*runState
	lock             *sync.RWMutex
}

type DatasetRequest struct {
	DatasetName string
	Since       string
	Limit       int64
}

type runState struct {
	consumer    *kafka.Consumer
	ctx         context.Context
	cancel      context.CancelFunc
	decoder     coder.Decoder
	isCancelled bool
}

func NewConsumers(lc fx.Lifecycle, env *conf.Env, mngr *conf.ConfigurationManager, statsd statsd.ClientInterface) (*Consumers, error) {
	config := &Consumers{
		env:              env,
		logger:           env.Logger.Named("consumers"),
		bootstrapServers: env.KafkaBrokers,
		mngr:             mngr,
		statsd:           statsd,
		running:          make(map[string]*runState),
		lock:             &sync.RWMutex{},
	}
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": strings.Join(config.bootstrapServers, ",")})
	if err != nil {
		return nil, err
	}
	config.adminClient = a
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			config.logger.Info("Registering Kafka consumers")
			config.logger.Info(env.KafkaBrokers)
			for _, c := range mngr.Datalayer.Consumers {
				config.add(c)
			}
			return nil
		},
		OnStop: nil,
	})

	return config, nil
}

func (consumers *Consumers) add(config conf.ConsumerConfig) {
	consumers.logger.Info("Got " + config.Topic)
}

func (consumers *Consumers) DoesDatasetExist(datasetName string) bool {
	for _, c := range consumers.mngr.Datalayer.Consumers {
		if c.Dataset == datasetName {
			return true
		}
	}
	return false
}

func (consumers *Consumers) config(datasetName string) *conf.ConsumerConfig {
	for _, c := range consumers.mngr.Datalayer.Consumers {
		if c.Dataset == datasetName {
			return &c
		}
	}
	return nil
}

func (consumers *Consumers) GetContext(datasetName string) map[string]interface{} {
	config := consumers.config(datasetName)

	ctx := make(map[string]interface{})
	namespaces := make(map[string]string)

	namespaces["ns0"] = config.BaseNameSpace + config.NameSpace + "/"
	namespaces["rdf"] = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	ctx["namespaces"] = namespaces
	ctx["id"] = "@context"
	return ctx
}

func (consumers *Consumers) ChangeSet(request DatasetRequest, callBack func(*coder.Entity)) error {
	config := consumers.config(request.DatasetName)
	if config == nil {
		return errors.New("config has disappeared, bad mojo")
	}

	tags := []string{
		fmt.Sprintf("application:%s", consumers.env.ServiceName),
		fmt.Sprintf("topic:%s", config.Topic),
	}

	// if multiple requests are made for the same groupId and topic, we need to make sure only one is running
	// at a time
	topicGroup := fmt.Sprintf("%s-%s", config.Topic, config.GroupId)
	consumers.lock.RLock()
	if cons, ok := consumers.running[topicGroup]; ok {
		// so, this means something is still running, so cancel it, and reset
		cons.cancel()
		if !cons.isCancelled {
			_ = cons.consumer.Close() // this should hopefully allow us to clean exit the previous consumer if still running
		}
	}
	consumers.lock.RUnlock()

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(consumers.bootstrapServers, ","),
		"group.id":           config.GroupId,
		"enable.auto.commit": false,
		"session.timeout.ms": 6000,
		"auto.offset.reset":  "earliest"})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())

	decoder, err := coder.NewDecoder(config)
	if err != nil {
		cancel()
		return err
	}
	state := &runState{
		consumer:    consumer,
		ctx:         ctx,
		cancel:      cancel,
		decoder:     decoder,
		isCancelled: false,
	}
	consumers.lock.Lock()
	consumers.running[topicGroup] = state
	consumers.lock.Unlock()

	// clean up, but there is a chance that this is never run
	defer func() {
		if !state.isCancelled {
			_ = state.consumer.Close()
		}
		state.cancel()
		state.isCancelled = true

		consumers.lock.Lock()
		delete(consumers.running, topicGroup)
		consumers.lock.Unlock()
	}()

	// so, if we get an offset, we need to reset the offsets now
	offsets := decodeSince(request.Since)
	consumers.logger.Debugf("supplied offsets via since token: %+v", offsets)
	err = consumers.resetOffsets(offsets, config, state.consumer)
	if err != nil {
		return err
	}

	err = state.consumer.Subscribe(config.Topic, nil)
	if err != nil {
		return err
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	run := true
	count := int64(0)

	partitionOffsets := make(map[int32]int64)

	nilCount := 0
	sinceCount := 0

	defer func() {
		if run {
			consumers.logger.Info("subscriber loop exited while state is still running. cancelling consumer context")
			state.cancel()
		}
	}()

	encoder := coder.NewEntityEncoder(config)
	isBeginning := true

	for run == true {
		select {
		case <-ctx.Done():
			consumers.logger.Debug("Terminating poll loop")
			run = false
		case sig := <-sigchan:
			consumers.logger.Infof("Caught signal %v: terminating", sig)
			run = false
		default:
			ev := state.consumer.Poll(500)

			switch e := ev.(type) {
			case *kafka.Message:
				count++
				nilCount = 0
				isBeginning = false
				sinceCount++
				partitionOffsets[e.TopicPartition.Partition] = int64(e.TopicPartition.Offset)
				value, err := state.decoder.Decode(e)
				if err != nil {
					consumers.logger.Warn(err)
					state.cancel()
				}

				var entity *coder.Entity
				var includeHeaders = &config.IncludeHeaders
				if *includeHeaders {
					entity = encoder.EncodeWithHeaders(e.Key, value, e.Headers)
				} else {
					entity = encoder.Encode(e.Key, value)
				}
				callBack(entity)
				if request.Limit > -1 && count >= request.Limit {
					consumers.logger.Debugf("reached requested limit of %v. stop poll loop", count)
					run = false
				}
				_ = consumers.statsd.Incr("kafka.read", tags, 1)
			case kafka.Error:
				// Errors should generally be considered
				// informational, the client will try to
				// automatically recover.
				// But in this example we choose to terminate
				// the application if all brokers are down.
				consumers.logger.Warn("poll returned a kafka.Error: ", e)
				if e.Code() == kafka.ErrAllBrokersDown {
					consumers.logger.Warn("Stopping consumer context due to kafka.Error")
					state.cancel()
				}
			default:
				if e == nil {
					nilCount++
					if isBeginning {
						if nilCount == 10 {
							consumers.logger.Debug("Got 10 nil counts, stopping. Do you need to increase the timeout?")
							state.cancel()
						}
						consumers.logger.Debug("Waiting for data")
						time.Sleep(1000 * time.Millisecond)
					} else {
						consumers.logger.Info("subscription depleted. cancelling consumer context")
						state.cancel()
					}

				}
			}
		}

	}

	if sinceCount > 0 {
		for p, offset := range offsets {
			if _, ok := partitionOffsets[p]; !ok {
				partitionOffsets[p] = offset
			}
		}
		s, _ := since(partitionOffsets)
		consumers.logger.Infof("Emitted %v msgs. offsets returned as @continuation: %+v (%+v)", sinceCount, partitionOffsets, s)
		entity := coder.NewEntity()
		entity.ID = "@continuation"
		entity.Properties["token"] = s

		callBack(entity)
	} else {
		if request.Since != "" {
			// there are no records, but we have since token, so just return that
			consumers.logger.Infof("nothing new found. returning since as @continuation: %+v", request.Since)
			entity := coder.NewEntity()
			entity.ID = "@continuation"
			entity.Properties["token"] = request.Since
			callBack(entity)
		}
	}
	//consumers.logger.Info(partitionOffsets)
	return nil
}

func (consumers *Consumers) resetOffsets(offsets map[int32]int64, config *conf.ConsumerConfig, c *kafka.Consumer) error {
	partitions := make([]kafka.TopicPartition, 0)
	for k, v := range offsets {
		consumers.logger.Infof("Resetting tp %d on %s to %d", k, config.Topic, v+1)
		partitions = append(partitions, kafka.TopicPartition{
			Topic:     &config.Topic,
			Partition: k,
			Offset:    kafka.Offset(v + 1),
		})
	}
	if len(partitions) > 0 {
		_, err := c.CommitOffsets(partitions)
		if err != nil {
			return err
		}
	} else {
		// we have no since token, reset to the start
		m, err := c.GetMetadata(&config.Topic, false, 1000)
		if err != nil {
			return err
		}
		for k, v := range m.Topics {
			if k == config.Topic {
				for _, p := range v.Partitions {
					consumers.logger.Infof("Resetting tp %d on %s to 0", p.ID, config.Topic)
					partitions = append(partitions, kafka.TopicPartition{
						Topic:     &config.Topic,
						Partition: p.ID,
						Offset:    kafka.Offset(0),
					})
				}
				_, err = c.CommitOffsets(partitions)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func since(partitions map[int32]int64) (string, error) {
	themBytes, err := json.Marshal(partitions)
	if err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(themBytes)
	return enc, nil
}

func decodeSince(since string) map[int32]int64 {
	paritionOffsets := make(map[int32]int64)
	if since == "" {
		return paritionOffsets
	}

	themBytes, err := base64.StdEncoding.DecodeString(since)
	if err != nil {
		return paritionOffsets
	}
	err = json.Unmarshal(themBytes, &paritionOffsets)
	if err != nil {
		return paritionOffsets
	}
	return paritionOffsets

}