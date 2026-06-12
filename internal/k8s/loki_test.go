package k8s

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestLokiClientListsLabelsAndSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loki/api/v1/labels":
			_, _ = w.Write([]byte(`{"status":"success","data":["app","namespace"]}`))
		case "/loki/api/v1/series":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"app":"api","namespace":"prod-app"},{"app":"web","namespace":"prod-app"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewLokiClient("test")
	client.BaseURL = server.URL

	labels, err := client.Labels(t.Context())
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if len(labels) != 2 || labels[1] != "namespace" {
		t.Fatalf("unexpected labels: %#v", labels)
	}

	series, err := client.Series(t.Context(), `{namespace=~"prod.*"}`)
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if len(series) != 2 || series[0]["app"] != "api" {
		t.Fatalf("unexpected series: %#v", series)
	}
}

func TestLokiClientQueriesRecentLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.URL.Path != "/loki/api/v1/query_range" || query.Get("direction") != "backward" || query.Get("limit") != "2" {
			t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		start, err := strconv.ParseInt(query.Get("start"), 10, 64)
		if err != nil {
			t.Fatalf("query_range must include start nanoseconds: %s", r.URL.RawQuery)
		}
		end, err := strconv.ParseInt(query.Get("end"), 10, 64)
		if err != nil {
			t.Fatalf("query_range must include end nanoseconds: %s", r.URL.RawQuery)
		}
		lookback := time.Duration(end - start)
		if lookback < 23*time.Hour || lookback > 25*time.Hour {
			t.Fatalf("query_range should cover roughly 24h, got %s from %s", lookback, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"stream":{"pod":"api-1"},"values":[["1710000000000000000","started"],["1709999999999999999","ready"]]}]}}`))
	}))
	defer server.Close()

	client := NewLokiClient("test")
	client.BaseURL = server.URL

	logs, err := client.QueryRange(t.Context(), `{namespace=~"prod.*"}`, 2)
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if len(logs) != 2 || logs[0].Line != "started" || logs[0].Stream["pod"] != "api-1" {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}
