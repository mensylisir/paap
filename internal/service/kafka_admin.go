package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"paap/internal/k8s"
)

type KafkaTopic struct {
	Name       string
	Partition  int
	Partitions int
}

type KafkaMessage struct {
	Topic     string
	Partition int
	Offset    int64
	Key       string
	Value     string
	Time      time.Time
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

func ReadKafkaMessages(ctx context.Context, info k8s.KafkaConnectionInfo, topic string, partition int, offset string, limit int) ([]KafkaMessage, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	partitions := []int{partition}
	if partition < 0 {
		ids, err := kafkaTopicPartitions(ctx, info, topic)
		if err != nil {
			return nil, err
		}
		partitions = ids
	}
	if len(partitions) == 0 {
		return nil, nil
	}

	messages := make([]KafkaMessage, 0, limit)
	for _, partitionID := range partitions {
		if len(messages) >= limit {
			break
		}
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:   []string{info.Broker},
			Topic:     topic,
			Partition: partitionID,
			MinBytes:  1,
			MaxBytes:  10e6,
			MaxWait:   500 * time.Millisecond,
		})
		if parsedOffset, ok, err := parseKafkaOffset(offset); err != nil {
			reader.Close()
			return nil, err
		} else if ok {
			if err := reader.SetOffset(parsedOffset); err != nil {
				reader.Close()
				return nil, err
			}
		} else if err := reader.SetOffset(kafka.FirstOffset); err != nil {
			reader.Close()
			return nil, err
		}

		readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		for len(messages) < limit {
			msg, err := reader.ReadMessage(readCtx)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					break
				}
				cancel()
				reader.Close()
				return nil, err
			}
			messages = append(messages, KafkaMessage{
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Key:       string(msg.Key),
				Value:     string(msg.Value),
				Time:      msg.Time,
			})
		}
		cancel()
		if err := reader.Close(); err != nil && len(messages) == 0 {
			return nil, err
		}
	}
	return messages, nil
}

func ProduceKafkaMessage(ctx context.Context, info k8s.KafkaConnectionInfo, topic, key, value string, partition int) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("topic is required")
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(info.Broker),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	defer writer.Close()
	message := kafka.Message{
		Key:   []byte(key),
		Value: []byte(value),
		Time:  time.Now(),
	}
	if partition >= 0 {
		message.Partition = partition
	}
	return writer.WriteMessages(ctx, message)
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

func kafkaTopicPartitions(ctx context.Context, info k8s.KafkaConnectionInfo, topic string) ([]int, error) {
	conn, err := kafka.DialContext(ctx, "tcp", info.Broker)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, err
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(partitions))
	for _, partition := range partitions {
		if partition.Topic != topic {
			continue
		}
		if _, ok := seen[partition.ID]; ok {
			continue
		}
		seen[partition.ID] = struct{}{}
		ids = append(ids, partition.ID)
	}
	sort.Ints(ids)
	return ids, nil
}

func parseKafkaOffset(value string) (int64, bool, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "first", "earliest", "beginning":
		return kafka.FirstOffset, true, nil
	case "last", "latest", "newest":
		return kafka.LastOffset, true, nil
	default:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			return 0, false, fmt.Errorf("offset must be first, latest, or a non-negative integer")
		}
		return parsed, true, nil
	}
}
