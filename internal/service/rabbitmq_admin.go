package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"paap/internal/k8s"
)

type RabbitMQQueue struct {
	Name     string `json:"name"`
	VHost    string `json:"vhost"`
	Messages int    `json:"messages"`
	State    string `json:"state"`
}

type RabbitMQExchange struct {
	Name  string `json:"name"`
	VHost string `json:"vhost"`
	Type  string `json:"type"`
}

type RabbitMQVHost struct {
	Name string `json:"name"`
}

type RabbitMQBinding struct {
	Source          string                 `json:"source"`
	VHost           string                 `json:"vhost"`
	Destination     string                 `json:"destination"`
	DestinationType string                 `json:"destination_type"`
	RoutingKey      string                 `json:"routing_key"`
	PropertiesKey   string                 `json:"properties_key"`
	Arguments       map[string]interface{} `json:"arguments"`
}

type RabbitMQMessage struct {
	Payload       string      `json:"payload"`
	PayloadBytes  int         `json:"payload_bytes"`
	PayloadString string      `json:"payload_string"`
	Exchange      string      `json:"exchange"`
	RoutingKey    string      `json:"routing_key"`
	MessageCount  int         `json:"message_count"`
	Properties    interface{} `json:"properties"`
	Redelivered   bool        `json:"redelivered"`
}

type RabbitMQPublishResult struct {
	Routed bool `json:"routed"`
}

func ListRabbitMQQueues(ctx context.Context, info k8s.RabbitMQConnectionInfo) ([]RabbitMQQueue, error) {
	var queues []RabbitMQQueue
	if err := rabbitMQJSON(ctx, info, http.MethodGet, "/api/queues", nil, &queues); err != nil {
		return nil, err
	}
	return queues, nil
}

func ListRabbitMQExchanges(ctx context.Context, info k8s.RabbitMQConnectionInfo) ([]RabbitMQExchange, error) {
	var exchanges []RabbitMQExchange
	if err := rabbitMQJSON(ctx, info, http.MethodGet, "/api/exchanges", nil, &exchanges); err != nil {
		return nil, err
	}
	return exchanges, nil
}

func ListRabbitMQVHosts(ctx context.Context, info k8s.RabbitMQConnectionInfo) ([]RabbitMQVHost, error) {
	var vhosts []RabbitMQVHost
	if err := rabbitMQJSON(ctx, info, http.MethodGet, "/api/vhosts", nil, &vhosts); err != nil {
		return nil, err
	}
	return vhosts, nil
}

func ListRabbitMQBindings(ctx context.Context, info k8s.RabbitMQConnectionInfo) ([]RabbitMQBinding, error) {
	var bindings []RabbitMQBinding
	if err := rabbitMQJSON(ctx, info, http.MethodGet, "/api/bindings", nil, &bindings); err != nil {
		return nil, err
	}
	return bindings, nil
}

func CreateRabbitMQQueue(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, queue string, durable bool) error {
	if vhost == "" {
		vhost = "/"
	}
	if queue == "" {
		return fmt.Errorf("queue is required")
	}
	payload := map[string]interface{}{"durable": durable}
	return rabbitMQJSON(ctx, info, http.MethodPut, "/api/queues/"+url.PathEscape(vhost)+"/"+url.PathEscape(queue), payload, nil)
}

func DeleteRabbitMQQueue(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, queue string) error {
	if vhost == "" {
		vhost = "/"
	}
	if queue == "" {
		return fmt.Errorf("queue is required")
	}
	return rabbitMQJSON(ctx, info, http.MethodDelete, "/api/queues/"+url.PathEscape(vhost)+"/"+url.PathEscape(queue), nil, nil)
}

func PurgeRabbitMQQueue(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, queue string) error {
	if vhost == "" {
		vhost = "/"
	}
	if queue == "" {
		return fmt.Errorf("queue is required")
	}
	return rabbitMQJSON(ctx, info, http.MethodDelete, "/api/queues/"+url.PathEscape(vhost)+"/"+url.PathEscape(queue)+"/contents", nil, nil)
}

func GetRabbitMQMessages(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, queue string, count int, requeue bool) ([]RabbitMQMessage, error) {
	if vhost == "" {
		vhost = "/"
	}
	if queue == "" {
		return nil, fmt.Errorf("queue is required")
	}
	if count <= 0 {
		count = 10
	}
	if count > 100 {
		count = 100
	}
	ackMode := "ack_requeue_false"
	if requeue {
		ackMode = "ack_requeue_true"
	}
	payload := map[string]interface{}{
		"count":    count,
		"ackmode":  ackMode,
		"encoding": "auto",
		"truncate": 50000,
	}
	var messages []RabbitMQMessage
	if err := rabbitMQJSON(ctx, info, http.MethodPost, "/api/queues/"+url.PathEscape(vhost)+"/"+url.PathEscape(queue)+"/get", payload, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func CreateRabbitMQExchange(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, exchange, exchangeType string, durable bool) error {
	if vhost == "" {
		vhost = "/"
	}
	if exchange == "" {
		return fmt.Errorf("exchange is required")
	}
	if exchangeType == "" {
		exchangeType = "direct"
	}
	payload := map[string]interface{}{
		"type":        exchangeType,
		"durable":     durable,
		"auto_delete": false,
		"internal":    false,
		"arguments":   map[string]interface{}{},
	}
	return rabbitMQJSON(ctx, info, http.MethodPut, "/api/exchanges/"+url.PathEscape(vhost)+"/"+url.PathEscape(exchange), payload, nil)
}

func DeleteRabbitMQExchange(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, exchange string) error {
	if vhost == "" {
		vhost = "/"
	}
	if exchange == "" {
		return fmt.Errorf("exchange is required")
	}
	return rabbitMQJSON(ctx, info, http.MethodDelete, "/api/exchanges/"+url.PathEscape(vhost)+"/"+url.PathEscape(exchange), nil, nil)
}

func CreateRabbitMQVHost(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost string) error {
	if vhost == "" {
		return fmt.Errorf("vhost is required")
	}
	return rabbitMQJSON(ctx, info, http.MethodPut, "/api/vhosts/"+url.PathEscape(vhost), map[string]interface{}{}, nil)
}

func DeleteRabbitMQVHost(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost string) error {
	if vhost == "" {
		return fmt.Errorf("vhost is required")
	}
	if vhost == "/" {
		return fmt.Errorf("default vhost cannot be deleted")
	}
	return rabbitMQJSON(ctx, info, http.MethodDelete, "/api/vhosts/"+url.PathEscape(vhost), nil, nil)
}

func CreateRabbitMQBinding(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, source, destinationType, destination, routingKey string, arguments map[string]interface{}) error {
	if vhost == "" {
		vhost = "/"
	}
	if source == "" {
		return fmt.Errorf("source exchange is required")
	}
	if destination == "" {
		return fmt.Errorf("destination is required")
	}
	destinationType = normalizeRabbitMQDestinationType(destinationType)
	if arguments == nil {
		arguments = map[string]interface{}{}
	}
	payload := map[string]interface{}{
		"routing_key": routingKey,
		"arguments":   arguments,
	}
	return rabbitMQJSON(ctx, info, http.MethodPost, "/api/bindings/"+url.PathEscape(vhost)+"/e/"+url.PathEscape(source)+"/"+destinationType+"/"+url.PathEscape(destination), payload, nil)
}

func DeleteRabbitMQBinding(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, source, destinationType, destination, propertiesKey string) error {
	if vhost == "" {
		vhost = "/"
	}
	if source == "" || destination == "" || propertiesKey == "" {
		return fmt.Errorf("source, destination and properties key are required")
	}
	destinationType = normalizeRabbitMQDestinationType(destinationType)
	return rabbitMQJSON(ctx, info, http.MethodDelete, "/api/bindings/"+url.PathEscape(vhost)+"/e/"+url.PathEscape(source)+"/"+destinationType+"/"+url.PathEscape(destination)+"/"+url.PathEscape(propertiesKey), nil, nil)
}

func PublishRabbitMQMessage(ctx context.Context, info k8s.RabbitMQConnectionInfo, vhost, exchange, routingKey, payload string, properties map[string]interface{}) (RabbitMQPublishResult, error) {
	if vhost == "" {
		vhost = "/"
	}
	if properties == nil {
		properties = map[string]interface{}{}
	}
	reqPayload := map[string]interface{}{
		"properties":       properties,
		"routing_key":      routingKey,
		"payload":          payload,
		"payload_encoding": "string",
	}
	var result RabbitMQPublishResult
	if err := rabbitMQJSON(ctx, info, http.MethodPost, "/api/exchanges/"+url.PathEscape(vhost)+"/"+url.PathEscape(exchange)+"/publish", reqPayload, &result); err != nil {
		return RabbitMQPublishResult{}, err
	}
	return result, nil
}

func normalizeRabbitMQDestinationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exchange", "e":
		return "e"
	default:
		return "q"
	}
}

func rabbitMQJSON(ctx context.Context, info k8s.RabbitMQConnectionInfo, method, path string, payload interface{}, target interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(info.ManagementURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(info.Username, info.Password)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return fmt.Errorf("rabbitmq API returned %d: %s", res.StatusCode, string(data))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(target)
}
