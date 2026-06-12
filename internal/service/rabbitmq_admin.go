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
