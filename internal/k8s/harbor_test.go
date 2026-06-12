package k8s

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHarborClientListsProjectsAndRepositories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "Harbor12345" {
			t.Fatalf("missing harbor basic auth")
		}
		switch r.URL.Path {
		case "/api/v2.0/projects":
			_, _ = w.Write([]byte(`[{"name":"billing"}]`))
		case "/api/v2.0/projects/billing/repositories":
			_, _ = w.Write([]byte(`[{"name":"billing/api","artifact_count":2},{"name":"billing/web","artifact_count":1}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHarborClient("test")
	client.BaseURL = server.URL

	projects, err := client.Projects(t.Context())
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "billing" {
		t.Fatalf("unexpected projects: %#v", projects)
	}

	repos, err := client.Repositories(t.Context(), "billing")
	if err != nil {
		t.Fatalf("repositories: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "billing/api" || repos[0].ArtifactCount != 2 {
		t.Fatalf("unexpected repos: %#v", repos)
	}
}

func TestHarborClientListsArtifactsAndTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/projects/billing/repositories/api/artifacts":
			_, _ = w.Write([]byte(`[{"digest":"sha256:abc","tags":[{"name":"1.0.0"},{"name":"prod"}]},{"digest":"sha256:def","tags":null}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHarborClient("test")
	client.BaseURL = server.URL

	artifacts, err := client.Artifacts(t.Context(), "billing", "api")
	if err != nil {
		t.Fatalf("artifacts: %v", err)
	}
	if len(artifacts) != 2 || artifacts[0].Digest != "sha256:abc" || len(artifacts[0].Tags) != 2 || artifacts[0].Tags[1].Name != "prod" {
		t.Fatalf("unexpected artifacts: %#v", artifacts)
	}
}

func TestHarborClientEnsuresProject(t *testing.T) {
	var sawCreate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "Harbor12345" {
			t.Fatalf("missing harbor basic auth")
		}
		switch r.URL.Path {
		case "/api/v2.0/projects/shop-dev":
			if r.Method != http.MethodGet {
				t.Fatalf("method for project lookup = %s, want GET", r.Method)
			}
			http.NotFound(w, r)
		case "/api/v2.0/projects":
			if r.Method != http.MethodPost {
				t.Fatalf("method for project create = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("content type = %q, want application/json", got)
			}
			sawCreate = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHarborClient("test")
	client.BaseURL = server.URL

	if err := client.EnsureProject(t.Context(), "shop-dev"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if !sawCreate {
		t.Fatalf("expected project create request")
	}
}

func TestHarborClientEnsureProjectAcceptsExistingProject(t *testing.T) {
	var sawCreate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/projects/shop-dev":
			w.WriteHeader(http.StatusOK)
		case "/api/v2.0/projects":
			sawCreate = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHarborClient("test")
	client.BaseURL = server.URL

	if err := client.EnsureProject(t.Context(), "shop-dev"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if sawCreate {
		t.Fatalf("existing project should not be created again")
	}
}

func TestHarborClientEnsureProjectTreatsConflictAsCreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/projects/shop-dev":
			http.NotFound(w, r)
		case "/api/v2.0/projects":
			w.WriteHeader(http.StatusConflict)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHarborClient("test")
	client.BaseURL = server.URL

	if err := client.EnsureProject(t.Context(), "shop-dev"); err != nil {
		t.Fatalf("ensure project should accept conflict: %v", err)
	}
}
