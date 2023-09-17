package main

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	kcli, err := kgo.NewClient(kgo.SeedBrokers(
		"kafka-0.kafka.netmux.svc.cluster.local:9092",
		"kafka-1.kafka.netmux.svc.cluster.local:9092",
		"kafka-2.kafka.netmux.svc.cluster.local:9092"),
		kgo.ConsumeTopics("topic01"),
		kgo.ProducerBatchCompression(kgo.GzipCompression()),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		return fmt.Errorf("error connecting to kafka: %w", err)
	}

	for {
		rec := kgo.StringRecord("Hello: " + time.Now().String())
		rec.Topic = "topic01"
		kcli.ProduceSync(context.Background(), rec)
		time.Sleep(time.Second * 2) //nolint:gomnd
	}
}
