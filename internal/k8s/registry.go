package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RegistryClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewRegistryClient(namespace string) *RegistryClient {
	fallback := fmt.Sprintf("http://%s.%s.svc.cluster.local:5000", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "registry", fallback)
	return &RegistryClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: http.DefaultTransport,
		},
	}
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
	escapedRepo := registryRepositoryPath(repository)
	if err := r.getJSON(ctx, "/v2/"+escapedRepo+"/tags/list", &payload); err != nil {
		return nil, err
	}
	return payload.Tags, nil
}

func (r *RegistryClient) ManifestDigest(ctx context.Context, repository, reference string) (string, error) {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	reference = strings.TrimSpace(reference)
	if repository == "" || reference == "" {
		return "", fmt.Errorf("repository and tag or digest are required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(r.BaseURL, "/")+"/v2/"+registryRepositoryPath(repository)+"/manifests/"+url.PathEscape(reference), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))
	res, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("registry manifest lookup failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry manifest lookup returned %d", res.StatusCode)
	}
	digest := strings.TrimSpace(res.Header.Get("Docker-Content-Digest"))
	if digest == "" {
		return "", fmt.Errorf("registry manifest response did not include Docker-Content-Digest")
	}
	return digest, nil
}

func (r *RegistryClient) ExposedPorts(ctx context.Context, repository, reference string) ([]int32, error) {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	reference = strings.TrimSpace(reference)
	if repository == "" || reference == "" {
		return nil, fmt.Errorf("repository and tag or digest are required")
	}
	var manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if err := r.getManifestJSON(ctx, repository, reference, &manifest); err != nil {
		return nil, err
	}
	if strings.TrimSpace(manifest.Config.Digest) == "" {
		return nil, fmt.Errorf("registry manifest response did not include config digest")
	}
	var imageConfig struct {
		Config struct {
			ExposedPorts map[string]any `json:"ExposedPorts"`
		} `json:"config"`
	}
	if err := r.getJSON(ctx, "/v2/"+registryRepositoryPath(repository)+"/blobs/"+url.PathEscape(manifest.Config.Digest), &imageConfig); err != nil {
		return nil, err
	}
	ports := make([]int, 0, len(imageConfig.Config.ExposedPorts))
	seen := map[int]struct{}{}
	for item := range imageConfig.Config.ExposedPorts {
		portText := strings.TrimSpace(strings.Split(item, "/")[0])
		port, err := strconv.Atoi(portText)
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	sort.Ints(ports)
	out := make([]int32, 0, len(ports))
	for _, port := range ports {
		out = append(out, int32(port))
	}
	return out, nil
}

func (r *RegistryClient) DeleteManifest(ctx context.Context, repository, digest string) error {
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	digest = strings.TrimSpace(digest)
	if repository == "" || digest == "" {
		return fmt.Errorf("repository and digest are required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(r.BaseURL, "/")+"/v2/"+registryRepositoryPath(repository)+"/manifests/"+url.PathEscape(digest), nil)
	if err != nil {
		return err
	}
	res, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("registry manifest delete failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	if len(body) > 0 {
		return fmt.Errorf("registry manifest delete returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("registry manifest delete returned %d", res.StatusCode)
}

func (r *RegistryClient) DeleteTag(ctx context.Context, repository, tag string) (string, error) {
	digest, err := r.ManifestDigest(ctx, repository, tag)
	if err != nil {
		return "", err
	}
	if err := r.DeleteManifest(ctx, repository, digest); err != nil {
		return digest, err
	}
	return digest, nil
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

func (r *RegistryClient) getManifestJSON(ctx context.Context, repository, reference string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(r.BaseURL, "/")+"/v2/"+registryRepositoryPath(repository)+"/manifests/"+url.PathEscape(reference), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
	}, ", "))
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

func registryRepositoryPath(repository string) string {
	return strings.ReplaceAll(url.PathEscape(strings.Trim(strings.TrimSpace(repository), "/")), "%2F", "/")
}
