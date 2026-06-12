package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/segmentio/kafka-go"

	"paap/internal/k8s"
)

type KafkaTopic struct {
	Name       string
	Partition  int
	Partitions int
}

func ListKafkaTopics(ctx context.Context, info k8s.KafkaConnectionInfo) ([]KafkaTopic, error) {
	conn, err := kafka.DialContext(ctx, "tcp", info.Broker)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, partition := range partitions {
		if partition.Topic == "" || partition.Topic[0] == '_' {
			continue
		}
		counts[partition.Topic]++
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	topics := make([]KafkaTopic, 0, len(names))
	for _, name := range names {
		topics = append(topics, KafkaTopic{Name: name, Partitions: counts[name]})
	}
	return topics, nil
}

func CreateKafkaTopic(ctx context.Context, info k8s.KafkaConnectionInfo, topic string, partitions int) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if partitions <= 0 {
		partitions = 1
	}
	client := &kafka.Client{
		Addr:    kafka.TCP(info.Broker),
		Timeout: 10 * time.Second,
	}
	_, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{
		Topics: []kafka.TopicConfig{{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: 1,
		}},
	})
	return err
}

func DeleteKafkaTopic(ctx context.Context, info k8s.KafkaConnectionInfo, topic string) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	client := &kafka.Client{
		Addr:    kafka.TCP(info.Broker),
		Timeout: 10 * time.Second,
	}
	_, err := client.DeleteTopics(ctx, &kafka.DeleteTopicsRequest{Topics: []string{topic}})
	return err
}
