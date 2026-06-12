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

type GiteaClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

type GiteaRepository struct {
	Name          string `json:"name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Stars         int    `json:"stars_count"`
	Forks         int    `json:"forks_count"`
	OpenIssues    int    `json:"open_issues_count"`
	UpdatedAt     string `json:"updated_at"`
	Language      string `json:"language"`
	Size          int    `json:"size"`
}

type GiteaContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	Encoding    string `json:"encoding"`
	Content     string `json:"content"`
	DownloadURL string `json:"download_url"`
}

type GiteaCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type GiteaCreateRepositoryRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Private       bool   `json:"private"`
	AutoInit      bool   `json:"auto_init"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

type GiteaKeyRequest struct {
	Title    string `json:"title"`
	Key      string `json:"key"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

func NewGiteaClient(namespace string) *GiteaClient {
	fallback := fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "git", fallback)
	return &GiteaClient{
		BaseURL:  baseURL,
		Username: "paap",
		Password: "paap123456",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (g *GiteaClient) Repositories(ctx context.Context) ([]GiteaRepository, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.BaseURL, "/")+"/api/v1/user/repos", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.Username, g.Password)
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitea API returned %d", res.StatusCode)
	}
	var repos []GiteaRepository
	if err := json.NewDecoder(res.Body).Decode(&repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (g *GiteaClient) CreateRepository(ctx context.Context, name, description string, private bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("gitea repository name is required")
	}
	payload := GiteaCreateRepositoryRequest{
		Name:          name,
		Description:   strings.TrimSpace(description),
		Private:       private,
		AutoInit:      true,
		DefaultBranch: "main",
	}
	status, body, err := g.doJSON(ctx, http.MethodPost, "/api/v1/user/repos", payload)
	if err != nil {
		return err
	}
	if status == http.StatusCreated || status == http.StatusOK || status == http.StatusConflict || status == http.StatusUnprocessableEntity {
		return nil
	}
	return fmt.Errorf("gitea create repository returned %d: %s", status, string(body))
}

func (g *GiteaClient) AddUserKey(ctx context.Context, title, key string) error {
	payload := GiteaKeyRequest{Title: strings.TrimSpace(title), Key: strings.TrimSpace(key)}
	if payload.Title == "" || payload.Key == "" {
		return fmt.Errorf("gitea key title and public key are required")
	}
	status, body, err := g.doJSON(ctx, http.MethodPost, "/api/v1/user/keys", payload)
	if err != nil {
		return err
	}
	if status == http.StatusCreated || status == http.StatusOK || status == http.StatusConflict || status == http.StatusUnprocessableEntity {
		return nil
	}
	return fmt.Errorf("gitea add user key returned %d: %s", status, string(body))
}

func (g *GiteaClient) AddDeployKey(ctx context.Context, repo, title, key string, readOnly bool) error {
	repo = strings.TrimSpace(repo)
	payload := GiteaKeyRequest{Title: strings.TrimSpace(title), Key: strings.TrimSpace(key), ReadOnly: readOnly}
	if repo == "" || payload.Title == "" || payload.Key == "" {
		return fmt.Errorf("gitea repository, key title and public key are required")
	}
	status, body, err := g.doJSON(ctx, http.MethodPost, "/api/v1/repos/paap/"+url.PathEscape(repo)+"/keys", payload)
	if err != nil {
		return err
	}
	if status == http.StatusCreated || status == http.StatusOK || status == http.StatusConflict || status == http.StatusUnprocessableEntity {
		return nil
	}
	return fmt.Errorf("gitea add deploy key returned %d: %s", status, string(body))
}

func (g *GiteaClient) RepositoryContents(ctx context.Context, repo, path, ref string) ([]GiteaContent, error) {
	repo = strings.TrimSpace(repo)
	path = strings.Trim(strings.TrimSpace(path), "/")
	if repo == "" {
		return nil, fmt.Errorf("gitea repository is required")
	}
	endpoint := strings.TrimRight(g.BaseURL, "/") + "/api/v1/repos/paap/" + url.PathEscape(repo) + "/contents"
	if path != "" {
		endpoint += "/" + giteaContentPath(path)
	}
	if strings.TrimSpace(ref) != "" {
		endpoint += "?ref=" + url.QueryEscape(strings.TrimSpace(ref))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.Username, g.Password)
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitea contents API returned %d", res.StatusCode)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}
	var items []GiteaContent
	if err := json.Unmarshal(raw, &items); err == nil {
		return items, nil
	}
	var item GiteaContent
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	items = append(items, item)
	return items, nil
}

func (g *GiteaClient) RepositoryCommits(ctx context.Context, repo, branch string, limit int) ([]GiteaCommit, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil, fmt.Errorf("gitea repository is required")
	}
	if limit <= 0 {
		limit = 5
	}
	endpoint := strings.TrimRight(g.BaseURL, "/") + "/api/v1/repos/paap/" + url.PathEscape(repo) + "/commits"
	query := url.Values{}
	if strings.TrimSpace(branch) != "" {
		query.Set("sha", strings.TrimSpace(branch))
	}
	query.Set("limit", fmt.Sprintf("%d", limit))
	endpoint += "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.Username, g.Password)
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitea commits API returned %d", res.StatusCode)
	}
	var commits []GiteaCommit
	if err := json.NewDecoder(res.Body).Decode(&commits); err != nil {
		return nil, err
	}
	return commits, nil
}

func giteaContentPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func (g *GiteaClient) doJSON(ctx context.Context, method, path string, payload interface{}) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(g.BaseURL, "/")+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(g.Username, g.Password)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data, nil
}
