package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PrometheusClient struct {
	BaseURLs   []string
	HTTPClient *http.Client
}

type PrometheusTarget struct {
	ScrapePool string            `json:"scrapePool"`
	Health     string            `json:"health"`
	Labels     map[string]string `json:"labels"`
	LastError  string            `json:"lastError"`
}

type PrometheusAlert struct {
	Labels   map[string]string `json:"labels"`
	State    string            `json:"state"`
	ActiveAt string            `json:"activeAt"`
}

type PrometheusRule struct {
	Group string
	Name  string
	Type  string
	State string
}

type PrometheusQuerySample struct {
	Metric map[string]string
	Value  float64
}

type PrometheusQueryPoint struct {
	Timestamp float64
	Value     float64
}

type PrometheusQuerySeries struct {
	Metric map[string]string
	Values []PrometheusQueryPoint
}

func NewPrometheusClient(namespace string) *PrometheusClient {
	fallback := fmt.Sprintf("http://prometheus-operated.%s.svc.cluster.local:9090", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	discovered := discoverServiceExact(ctx, namespace, "-prometheus", fallback)
	urls := []string{discovered}
	if discovered != fallback {
		urls = append(urls, fallback)
	}
	urls = append(urls,
		fmt.Sprintf("http://%s-prometheus.%s.svc.cluster.local:9090", namespace, namespace),
	)
	return &PrometheusClient{
		BaseURLs:   urls,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PrometheusClient) Targets(ctx context.Context) ([]PrometheusTarget, error) {
	var payload struct {
		Data struct {
			ActiveTargets []PrometheusTarget `json:"activeTargets"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, "/api/v1/targets", &payload); err != nil {
		return nil, err
	}
	return payload.Data.ActiveTargets, nil
}

func (p *PrometheusClient) Alerts(ctx context.Context) ([]PrometheusAlert, error) {
	var payload struct {
		Data struct {
			Alerts []PrometheusAlert `json:"alerts"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, "/api/v1/alerts", &payload); err != nil {
		return nil, err
	}
	return payload.Data.Alerts, nil
}

func (p *PrometheusClient) Rules(ctx context.Context) ([]PrometheusRule, error) {
	var payload struct {
		Data struct {
			Groups []struct {
				Name  string `json:"name"`
				Rules []struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					State string `json:"state"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, "/api/v1/rules", &payload); err != nil {
		return nil, err
	}
	rules := make([]PrometheusRule, 0)
	for _, group := range payload.Data.Groups {
		for _, rule := range group.Rules {
			rules = append(rules, PrometheusRule{
				Group: group.Name,
				Name:  rule.Name,
				Type:  rule.Type,
				State: rule.State,
			})
		}
	}
	return rules, nil
}

func (p *PrometheusClient) Query(ctx context.Context, query string) ([]PrometheusQuerySample, error) {
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, "/api/v1/query?query="+url.QueryEscape(query), &payload); err != nil {
		return nil, err
	}
	out := make([]PrometheusQuerySample, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		if len(item.Value) < 2 {
			continue
		}
		valueText, ok := item.Value[1].(string)
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(valueText, 64)
		if err != nil {
			continue
		}
		out = append(out, PrometheusQuerySample{Metric: item.Metric, Value: value})
	}
	return out, nil
}

func (p *PrometheusClient) QueryRange(ctx context.Context, query string, start time.Time, end time.Time, step time.Duration) ([]PrometheusQuerySeries, error) {
	if step <= 0 {
		step = time.Minute
	}
	values := url.Values{}
	values.Set("query", query)
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", strconv.FormatInt(int64(step.Seconds()), 10))
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := p.getJSON(ctx, "/api/v1/query_range?"+values.Encode(), &payload); err != nil {
		return nil, err
	}
	out := make([]PrometheusQuerySeries, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		points := make([]PrometheusQueryPoint, 0, len(item.Values))
		for _, pair := range item.Values {
			if len(pair) < 2 {
				continue
			}
			ts, ok := prometheusFloat(pair[0])
			if !ok {
				continue
			}
			valueText, ok := pair[1].(string)
			if !ok {
				continue
			}
			value, err := strconv.ParseFloat(valueText, 64)
			if err != nil {
				continue
			}
			points = append(points, PrometheusQueryPoint{Timestamp: ts, Value: value})
		}
		if len(points) > 0 {
			out = append(out, PrometheusQuerySeries{Metric: item.Metric, Values: points})
		}
	}
	return out, nil
}

func prometheusFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (a PrometheusAlert) Name() string {
	if a.Labels == nil {
		return "alert"
	}
	if name := a.Labels["alertname"]; name != "" {
		return name
	}
	return "alert"
}

func (p *PrometheusClient) getJSON(ctx context.Context, path string, target interface{}) error {
	var lastErr error
	for _, baseURL := range p.BaseURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
		if err != nil {
			return err
		}
		res, err := p.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if res.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("prometheus API returned %d: %s", res.StatusCode, string(body))
			continue
		}
		if err := json.Unmarshal(body, target); err != nil {
			return err
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no prometheus base URLs configured")
	}
	return lastErr
}
