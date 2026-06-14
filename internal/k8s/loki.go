package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LokiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type LokiLogEntry struct {
	Stream    map[string]string
	Timestamp string
	Line      string
}

func NewLokiClient(namespace string) *LokiClient {
	fallback := fmt.Sprintf("http://%s.%s.svc.cluster.local:3100", defaultLokiServiceName(namespace), namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "loki", fallback)
	return &LokiClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func defaultLokiServiceName(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if strings.HasSuffix(namespace, "-loki") {
		return namespace
	}
	return namespace + "-loki"
}

func (l *LokiClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(l.BaseURL, "/")+"/ready", nil)
	if err != nil {
		return err
	}
	res, err := l.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("loki health check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("loki health check returned %d", res.StatusCode)
	}
	return nil
}

func (l *LokiClient) Labels(ctx context.Context) ([]string, error) {
	var payload struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := l.getJSON(ctx, "/loki/api/v1/labels", &payload); err != nil {
		return nil, err
	}
	if payload.Status != "" && payload.Status != "success" {
		return nil, fmt.Errorf("loki labels returned status %s", payload.Status)
	}
	return payload.Data, nil
}

func (l *LokiClient) Series(ctx context.Context, match string) ([]map[string]string, error) {
	values := url.Values{}
	if strings.TrimSpace(match) != "" {
		values.Set("match[]", match)
	}
	path := "/loki/api/v1/series"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var payload struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}
	if err := l.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "" && payload.Status != "success" {
		return nil, fmt.Errorf("loki series returned status %s", payload.Status)
	}
	return payload.Data, nil
}

func (l *LokiClient) QueryRange(ctx context.Context, query string, limit int) ([]LokiLogEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	end := time.Now()
	start := end.Add(-24 * time.Hour)
	values := url.Values{}
	values.Set("query", query)
	values.Set("direction", "backward")
	values.Set("limit", strconv.Itoa(limit))
	values.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	values.Set("end", strconv.FormatInt(end.UnixNano(), 10))

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := l.getJSON(ctx, "/loki/api/v1/query_range?"+values.Encode(), &payload); err != nil {
		return nil, err
	}
	if payload.Status != "" && payload.Status != "success" {
		return nil, fmt.Errorf("loki query_range returned status %s", payload.Status)
	}
	entries := make([]LokiLogEntry, 0)
	for _, result := range payload.Data.Result {
		for _, value := range result.Values {
			if len(value) < 2 {
				continue
			}
			entries = append(entries, LokiLogEntry{
				Stream:    result.Stream,
				Timestamp: value[0],
				Line:      value[1],
			})
		}
	}
	return entries, nil
}

func (l *LokiClient) getJSON(ctx context.Context, path string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(l.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	res, err := l.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("loki API returned %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
