package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiteaClientListsRepositories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		if r.URL.Path != "/api/v1/user/repos" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"name":"billing-dev-components","html_url":"http://gitea/paap/billing-dev-components","clone_url":"http://gitea/paap/billing-dev-components.git","default_branch":"main"}]`))
	}))
	defer server.Close()

	client := NewGiteaClient("test")
	client.BaseURL = server.URL

	repos, err := client.Repositories(t.Context())
	if err != nil {
		t.Fatalf("repositories: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "billing-dev-components" || repos[0].DefaultBranch != "main" {
		t.Fatalf("unexpected repositories: %#v", repos)
	}
}

func TestGiteaClientListsRepositoryContents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		if r.URL.Path != "/api/v1/repos/paap/billing-dev-components/contents/components/api" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("ref") != "main" {
			t.Fatalf("ref = %q, want main", r.URL.Query().Get("ref"))
		}
		_, _ = w.Write([]byte(`[
			{"name":"README.md","path":"components/api/README.md","type":"file"},
			{"name":"Jenkinsfile","path":"components/api/Jenkinsfile","type":"file"},
			{"name":"deployment.yaml","path":"components/api/deployment.yaml","type":"file"}
		]`))
	}))
	defer server.Close()

	client := NewGiteaClient("test")
	client.BaseURL = server.URL

	items, err := client.RepositoryContents(t.Context(), "billing-dev-components", "components/api", "main")
	if err != nil {
		t.Fatalf("repository contents: %v", err)
	}
	if len(items) != 3 || items[0].Name != "README.md" || items[0].Type != "file" {
		t.Fatalf("unexpected contents: %#v", items)
	}
}

func TestGiteaClientReadsRepositoryFileContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		if r.URL.Path != "/api/v1/repos/paap/billing-dev-components/contents/components/api/README.md" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"name":"README.md",
			"path":"components/api/README.md",
			"type":"file",
			"size":18,
			"encoding":"base64",
			"content":"IyBBUEkKCi1idWlsZC1kZXBsb3kK",
			"download_url":"http://gitea/raw/README.md"
		}`))
	}))
	defer server.Close()

	client := NewGiteaClient("test")
	client.BaseURL = server.URL

	items, err := client.RepositoryContents(t.Context(), "billing-dev-components", "components/api/README.md", "main")
	if err != nil {
		t.Fatalf("repository file content: %v", err)
	}
	if len(items) != 1 || items[0].Type != "file" || items[0].Content == "" || items[0].Encoding != "base64" || items[0].Size != 18 {
		t.Fatalf("unexpected file content: %#v", items)
	}
}

func TestGiteaClientListsRepositoryCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		if r.URL.Path != "/api/v1/repos/paap/billing-dev-components/commits" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("sha") != "main" {
			t.Fatalf("sha = %q, want main", r.URL.Query().Get("sha"))
		}
		if r.URL.Query().Get("limit") != "2" {
			t.Fatalf("limit = %q, want 2", r.URL.Query().Get("limit"))
		}
		_, _ = w.Write([]byte(`[
			{"sha":"abcdef123456","commit":{"message":"sync deployment","author":{"name":"PAAP","date":"2026-06-04T10:00:00Z"}}},
			{"sha":"123456abcdef","commit":{"message":"init repo","author":{"name":"Admin","date":"2026-06-04T09:00:00Z"}}}
		]`))
	}))
	defer server.Close()

	client := NewGiteaClient("test")
	client.BaseURL = server.URL

	commits, err := client.RepositoryCommits(t.Context(), "billing-dev-components", "main", 2)
	if err != nil {
		t.Fatalf("repository commits: %v", err)
	}
	if len(commits) != 2 || commits[0].SHA != "abcdef123456" || commits[0].Commit.Message != "sync deployment" || commits[0].Commit.Author.Name != "PAAP" {
		t.Fatalf("unexpected commits: %#v", commits)
	}
}
