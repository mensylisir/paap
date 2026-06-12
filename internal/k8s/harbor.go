package k8s

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
)

type HarborClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

type HarborProject struct {
	Name string `json:"name"`
}

type harborProjectCreateRequest struct {
	ProjectName string `json:"project_name"`
	Public      bool   `json:"public"`
}

type HarborRepository struct {
	Name          string `json:"name"`
	ArtifactCount int    `json:"artifact_count"`
}

type HarborArtifact struct {
	Digest string      `json:"digest"`
	Tags   []HarborTag `json:"tags"`
}

type HarborTag struct {
	Name string `json:"name"`
}

func NewHarborClient(namespace string) *HarborClient {
	fallback := fmt.Sprintf("http://harbor-core.%s.svc.cluster.local", namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "harbor", fallback)
	return &HarborClient{
		BaseURL:  baseURL,
		Username: "admin",
		Password: "Harbor12345",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (h *HarborClient) HealthCheck(ctx context.Context) error {
	req, err := h.newRequest(ctx, http.MethodGet, "/api/v2.0/health")
	if err != nil {
		return err
	}
	res, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("harbor health check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("harbor health check returned %d", res.StatusCode)
	}
	return nil
}

func (h *HarborClient) Projects(ctx context.Context) ([]HarborProject, error) {
	var projects []HarborProject
	if err := h.getJSON(ctx, "/api/v2.0/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (h *HarborClient) EnsureProject(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("harbor project name is required")
	}
	path := "/api/v2.0/projects/" + url.PathEscape(name)
	req, err := h.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	res, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("harbor project lookup failed: %w", err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("harbor project lookup returned %d", res.StatusCode)
	}

	payload, err := json.Marshal(harborProjectCreateRequest{ProjectName: name, Public: false})
	if err != nil {
		return err
	}
	req, err = h.newRequest(ctx, http.MethodPost, "/api/v2.0/projects")
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(payload))
	req.ContentLength = int64(len(payload))
	res, err = h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("harbor project create failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusCreated || res.StatusCode == http.StatusConflict {
		return nil
	}
	return fmt.Errorf("harbor project create returned %d", res.StatusCode)
}

func (h *HarborClient) Repositories(ctx context.Context, project string) ([]HarborRepository, error) {
	var repos []HarborRepository
	path := "/api/v2.0/projects/" + url.PathEscape(project) + "/repositories"
	if err := h.getJSON(ctx, path, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (h *HarborClient) Artifacts(ctx context.Context, project, repository string) ([]HarborArtifact, error) {
	var artifacts []HarborArtifact
	repository = strings.TrimPrefix(repository, project+"/")
	path := "/api/v2.0/projects/" + url.PathEscape(project) + "/repositories/" + harborPathSegment(repository) + "/artifacts"
	if err := h.getJSON(ctx, path, &artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (h *HarborClient) getJSON(ctx context.Context, path string, target interface{}) error {
	req, err := h.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	res, err := h.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("harbor API returned %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func (h *HarborClient) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(h.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(h.Username, h.Password)
	return req, nil
}

func harborPathSegment(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "%2F", "/")
}
