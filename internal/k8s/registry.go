package k8s

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RegistryClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewRegistryClient(namespace string) *RegistryClient {
	fallback := fmt.Sprintf("https://%s.%s.svc.cluster.local:5000", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "registry", fallback)
	return &RegistryClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: registryHTTPTransport(baseURL),
		},
	}
}

func registryHTTPTransport(baseURL string) http.RoundTripper {
	if strings.HasPrefix(baseURL, "https://") && strings.Contains(baseURL, ".svc.cluster.local") {
		return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	return http.DefaultTransport
}

func (r *RegistryClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(r.BaseURL, "/")+"/v2/", nil)
	if err != nil {
		return err
	}
	res, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("registry health check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("registry health check returned %d", res.StatusCode)
	}
	return nil
}

func (r *RegistryClient) Catalog(ctx context.Context) ([]string, error) {
	var payload struct {
		Repositories []string `json:"repositories"`
	}
	if err := r.getJSON(ctx, "/v2/_catalog", &payload); err != nil {
		return nil, err
	}
	return payload.Repositories, nil
}

func (r *RegistryClient) Tags(ctx context.Context, repository string) ([]string, error) {
	var payload struct {
		Tags []string `json:"tags"`
	}
	escapedRepo := strings.ReplaceAll(url.PathEscape(repository), "%2F", "/")
	if err := r.getJSON(ctx, "/v2/"+escapedRepo+"/tags/list", &payload); err != nil {
		return nil, err
	}
	return payload.Tags, nil
}

func (r *RegistryClient) getJSON(ctx context.Context, path string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(r.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	res, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("registry API returned %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
