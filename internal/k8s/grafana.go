package k8s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GrafanaClient interacts with a Grafana instance's HTTP API.
type GrafanaClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewGrafanaClient creates a client for the Grafana instance in the given namespace.
// Uses the in-cluster service DNS: http://grafana.{namespace}.svc.cluster.local:3000
func NewGrafanaClient(namespace string) *GrafanaClient {
	return &GrafanaClient{
		BaseURL: fmt.Sprintf("http://grafana.%s.svc.cluster.local:3000", namespace),
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
	req.SetBasicAuth("admin", "admin")

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
