package main

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"go.uber.org/zap"

	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/conf"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/kafka"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/security"
	"github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/web"
)

func main() {
	envConf := conf.NewEnv()

	logger := conf.NewLogger(envConf)

	statsdClient, _ := conf.NewStatsd(envConf)

	providers := security.NewTokenProviders(logger)

	mngr := conf.NewConfigurationManager(envConf, providers)

	kafkaService, _ := kafka.NewService(envConf, logger, mngr, statsdClient)

	webServer, _ := web.NewWebServer(
		envConf,
		kafkaService,
		logger,
		statsdClient)

	handleShutdown(logger, webServer, kafkaService)
}

type Stoppable interface {
	Shutdown(ctx context.Context) error
}

func handleShutdown(logger *zap.SugaredLogger, stoppables ...Stoppable) {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan
	logger.Infof("Application stopping!")

	shutdownCtx := context.Background()
	wg := sync.WaitGroup{}
	for _, s := range stoppables {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Shutdown(shutdownCtx)
			if err != nil {
				logger.Errorf("Stopping Application failed: %+v", err)
				os.Exit(2)
			}
		}()
	}
	wg.Wait()
	logger.Infof("Application stopped!")
	os.Exit(0)
}