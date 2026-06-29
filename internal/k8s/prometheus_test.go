package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrometheusClientReadsTargetsAlertsAndRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/targets":
			_, _ = w.Write([]byte(`{"status":"success","data":{"activeTargets":[{"scrapePool":"pods","health":"up","labels":{"job":"api","namespace":"billing-dev"}}]}}`))
		case "/api/v1/alerts":
			_, _ = w.Write([]byte(`{"status":"success","data":{"alerts":[{"labels":{"alertname":"HighErrorRate"},"state":"firing","activeAt":"2026-06-03T00:00:00Z"}]}}`))
		case "/api/v1/rules":
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[{"name":"paap","rules":[{"name":"ErrorBudget","type":"alerting","state":"inactive"}]}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := &PrometheusClient{BaseURLs: []string{server.URL}, HTTPClient: server.Client()}
	targets, err := client.Targets(t.Context())
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	if len(targets) != 1 || targets[0].ScrapePool != "pods" || targets[0].Health != "up" || targets[0].Labels["job"] != "api" {
		t.Fatalf("unexpected targets %#v", targets)
	}

	alerts, err := client.Alerts(t.Context())
	if err != nil {
		t.Fatalf("alerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Name() != "HighErrorRate" || alerts[0].State != "firing" {
		t.Fatalf("unexpected alerts %#v", alerts)
	}

	rules, err := client.Rules(t.Context())
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	if len(rules) != 1 || rules[0].Name != "ErrorBudget" || rules[0].Group != "paap" {
		t.Fatalf("unexpected rules %#v", rules)
	}
}

func TestPrometheusClientQueryRangeReadsTimeSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		if query.Get("query") != "sum(rate(container_cpu_usage_seconds_total[5m]))" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if query.Get("start") == "" || query.Get("end") == "" || query.Get("step") != "60" {
			t.Fatalf("query_range must include start/end/step seconds: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"metric":{"pod":"api-1"},"values":[[1710000000,"0.1"],[1710000060,"0.2"]]},{"metric":{"pod":"api-2"},"values":[[1710000000,"0.3"],[1710000060,"0.4"]]}]}}`))
	}))
	t.Cleanup(server.Close)

	client := &PrometheusClient{BaseURLs: []string{server.URL}, HTTPClient: server.Client()}
	series, err := client.QueryRange(
		t.Context(),
		"sum(rate(container_cpu_usage_seconds_total[5m]))",
		time.Unix(1710000000, 0),
		time.Unix(1710000060, 0),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if len(series) != 2 || series[0].Metric["pod"] != "api-1" || len(series[0].Values) != 2 {
		t.Fatalf("unexpected series %#v", series)
	}
	if series[1].Values[1].Timestamp != 1710000060 || series[1].Values[1].Value != 0.4 {
		t.Fatalf("unexpected second series point %#v", series[1].Values[1])
	}
}
