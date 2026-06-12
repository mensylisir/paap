package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
