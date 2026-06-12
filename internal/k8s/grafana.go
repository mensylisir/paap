package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GrafanaClient interacts with a Grafana instance's HTTP API.
type GrafanaClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewGrafanaClient creates a client for the Grafana instance in the given namespace.
func NewGrafanaClient(namespace string) *GrafanaClient {
	fallback := fmt.Sprintf("http://%s-grafana.%s.svc.cluster.local", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "grafana", fallback)
	user, pass := discoverGrafanaCreds(ctx, namespace)
	return &GrafanaClient{
		BaseURL:  baseURL,
		Username: user,
		Password: pass,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DashboardRequest is the payload for Grafana's dashboard import API.
type DashboardRequest struct {
	Dashboard json.RawMessage `json:"dashboard"`
	Overwrite bool            `json:"overwrite"`
	Message   string          `json:"message,omitempty"`
}

type GrafanaDashboard struct {
	Title string   `json:"title"`
	UID   string   `json:"uid"`
	URL   string   `json:"url"`
	Tags  []string `json:"tags"`
}

type grafanaDatasourceRequest struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	UID       string                 `json:"uid,omitempty"`
	Access    string                 `json:"access"`
	URL       string                 `json:"url"`
	IsDefault bool                   `json:"isDefault"`
	JSONData  map[string]interface{} `json:"jsonData,omitempty"`
}

func (g *GrafanaClient) Dashboards(ctx context.Context) ([]GrafanaDashboard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.BaseURL, "/")+"/api/search?type=dash-db", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.Username, g.Password)
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grafana dashboard search failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("grafana dashboard search returned %d: %s", res.StatusCode, string(body))
	}
	var dashboards []GrafanaDashboard
	if err := json.NewDecoder(res.Body).Decode(&dashboards); err != nil {
		return nil, err
	}
	return dashboards, nil
}

func (g *GrafanaClient) EnsureLokiDatasource(ctx context.Context, lokiURL string) error {
	lokiURL = strings.TrimSpace(lokiURL)
	if lokiURL == "" {
		return fmt.Errorf("loki datasource URL is required")
	}
	baseURL := strings.TrimRight(g.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/datasources/name/Loki", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(g.Username, g.Password)
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("grafana datasource lookup failed: %w", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("grafana datasource lookup returned %d: %s", res.StatusCode, string(body))
	}

	payload := grafanaDatasourceRequest{
		Name:      "Loki",
		Type:      "loki",
		UID:       "loki",
		Access:    "proxy",
		URL:       lokiURL,
		IsDefault: false,
		JSONData: map[string]interface{}{
			"maxLines": 1000,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/datasources", bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(g.Username, g.Password)
	res, err = g.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("grafana datasource create failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("grafana datasource create returned %d: %s", res.StatusCode, string(body))
	}
	return nil
}

// ImportDashboard imports a Grafana dashboard JSON into the Grafana instance.
// The dashboardJSON should be a valid Grafana dashboard JSON object.
func (g *GrafanaClient) ImportDashboard(dashboardJSON string, title string) error {
	// Parse and ensure the dashboard has required fields
	var dashboard map[string]interface{}
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		return fmt.Errorf("invalid dashboard JSON: %w", err)
	}

	// Remove id to let Grafana assign one
	delete(dashboard, "id")

	// Set uid if not present to enable overwrite
	if _, ok := dashboard["uid"]; !ok {
		dashboard["uid"] = fmt.Sprintf("paap-auto-%d", time.Now().UnixNano())
	}

	dashboardBytes, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	payload := DashboardRequest{
		Dashboard: dashboardBytes,
		Overwrite: true,
		Message:   fmt.Sprintf("Auto-provisioned by PAAP: %s", title),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", g.BaseURL+"/api/dashboards/db", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(g.Username, g.Password)

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("grafana API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("grafana API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// HealthCheck checks if Grafana is reachable.
func (g *GrafanaClient) HealthCheck() error {
	resp, err := g.HTTPClient.Get(g.BaseURL + "/api/health")
	if err != nil {
		return fmt.Errorf("grafana health check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana health check returned %d", resp.StatusCode)
	}
	return nil
}
