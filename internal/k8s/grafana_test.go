package k8s

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGrafanaClientListsDashboards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin" {
			t.Fatalf("missing grafana basic auth")
		}
		if r.URL.Path != "/api/search" || r.URL.Query().Get("type") != "dash-db" {
			t.Fatalf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"title":"PAAP Overview","uid":"paap-overview","url":"/d/paap-overview","tags":["paap","prod"]}]`))
	}))
	defer server.Close()

	client := NewGrafanaClient("test")
	client.BaseURL = server.URL

	dashboards, err := client.Dashboards(t.Context())
	if err != nil {
		t.Fatalf("dashboards: %v", err)
	}
	if len(dashboards) != 1 || dashboards[0].Title != "PAAP Overview" || dashboards[0].URL != "/d/paap-overview" {
		t.Fatalf("unexpected dashboards: %#v", dashboards)
	}
}

func TestGrafanaClientEnsureLokiDatasourceCreatesMissingDatasource(t *testing.T) {
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin" {
			t.Fatalf("missing grafana basic auth")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/datasources/name/Loki":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/datasources":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode datasource payload: %v", err)
			}
			if payload["name"] != "Loki" || payload["type"] != "loki" || payload["url"] != "http://loki:3100" || payload["access"] != "proxy" {
				t.Fatalf("unexpected datasource payload: %#v", payload)
			}
			created = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"datasource":{"id":3}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewGrafanaClient("test")
	client.BaseURL = server.URL

	if err := client.EnsureLokiDatasource(t.Context(), "http://loki:3100"); err != nil {
		t.Fatalf("ensure loki datasource: %v", err)
	}
	if !created {
		t.Fatalf("loki datasource was not created")
	}
}

func TestGrafanaClientEnsureLokiDatasourceUpdatesWrongExistingDatasource(t *testing.T) {
	updated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin" {
			t.Fatalf("missing grafana basic auth")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/datasources/name/Loki":
			_, _ = w.Write([]byte(`{"id":7,"uid":"loki","name":"Loki","type":"loki","access":"proxy","url":"http://wrong-loki:3100"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/datasources/7":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode datasource payload: %v", err)
			}
			if payload["url"] != "http://real-loki:3100" || payload["type"] != "loki" || payload["access"] != "proxy" {
				t.Fatalf("unexpected datasource update payload: %#v", payload)
			}
			updated = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"datasource":{"id":7}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewGrafanaClient("test")
	client.BaseURL = server.URL

	if err := client.EnsureLokiDatasource(t.Context(), "http://real-loki:3100"); err != nil {
		t.Fatalf("ensure loki datasource: %v", err)
	}
	if !updated {
		t.Fatalf("loki datasource was not updated")
	}
}
