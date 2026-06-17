package k8s

import (
	"context"
	"fmt"
	"strings"
)

type MongoDBConnectionInfo struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

type RabbitMQConnectionInfo struct {
	ManagementURL string
	Username      string
	Password      string
}

type KafkaConnectionInfo struct {
	Broker        string
	Username      string
	Password      string
	SASLMechanism string
}

func DiscoverMongoDBConnection(ctx context.Context, namespace string) (MongoDBConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return MongoDBConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, 27017)
	if err != nil {
		return MongoDBConnectionInfo{}, err
	}
	password, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"mongodb-root-password", "mongodb-password", "password"})
	return MongoDBConnectionInfo{
		Host:     host,
		Port:     27017,
		Username: "root",
		Password: password,
		Database: "admin",
	}, nil
}

func DiscoverRabbitMQConnection(ctx context.Context, namespace string) (RabbitMQConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return RabbitMQConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, 15672)
	if err != nil {
		return RabbitMQConnectionInfo{}, err
	}
	username, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"rabbitmq-username", "username"})
	password, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"rabbitmq-password", "password"})
	if username == "" {
		username = "user"
	}
	if password == "" {
		password = "changeme123"
	}
	return RabbitMQConnectionInfo{
		ManagementURL: fmt.Sprintf("http://%s:%d", host, 15672),
		Username:      username,
		Password:      password,
	}, nil
}

func DiscoverKafkaConnection(ctx context.Context, namespace string) (KafkaConnectionInfo, error) {
	cl, err := requireClient()
	if err != nil {
		return KafkaConnectionInfo{}, err
	}
	host, err := discoverServiceHost(ctx, cl, namespace, 9092)
	if err != nil {
		return KafkaConnectionInfo{}, err
	}
	password, _ := discoverOptionalSecretValue(ctx, cl, namespace, []string{"client-passwords", "kafka-password", "password"})
	password = firstSecretListValue(password)
	info := KafkaConnectionInfo{Broker: fmt.Sprintf("%s:%d", host, 9092)}
	if password != "" {
		info.Username = "user1"
		info.Password = password
		info.SASLMechanism = "PLAIN"
	}
	return info, nil
}

func firstSecretListValue(value string) string {
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
