package k8s

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryClientListsCatalogAndTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/_catalog":
			_, _ = w.Write([]byte(`{"repositories":["billing/api","billing/web"]}`))
		case "/v2/billing/api/tags/list":
			_, _ = w.Write([]byte(`{"name":"billing/api","tags":["v1","v2"]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewRegistryClient("test")
	client.BaseURL = server.URL

	repos, err := client.Catalog(t.Context())
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(repos) != 2 || repos[0] != "billing/api" || repos[1] != "billing/web" {
		t.Fatalf("unexpected repos: %#v", repos)
	}

	tags, err := client.Tags(t.Context(), "billing/api")
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "v1" || tags[1] != "v2" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}
