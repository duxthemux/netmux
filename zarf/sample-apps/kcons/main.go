package main

import (
	"context"
	"fmt"
	"log"
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
		kgo.ConsumerGroup("cons-01"),
		kgo.BlockRebalanceOnPoll(),
		kgo.FetchMaxWait(time.Second*3), //nolint:gomnd
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return fmt.Errorf("error connecting to kafka: %w", err)
	}

	err = kcli.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("error pinging kafka: %w", err)
	}

	for {
		fetches := kcli.PollRecords(context.Background(), 3) //nolint:gomnd
		if fetches.IsClientClosed() {
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("got fetch errors: %v", errs)
		}

		if recs := fetches.Records(); len(recs) > 0 {
			for _, rec := range fetches.Records() {
				log.Print(string(rec.Value))
			}

			kcli.AllowRebalance()
		}
	}
}
