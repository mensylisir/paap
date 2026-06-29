package k8s

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestJenkinsClientListsJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"jobs":[{"name":"api-build","url":"http://jenkins/job/api-build/","color":"blue"},{"name":"web-build","url":"http://jenkins/job/web-build/","color":"red"}]}`))
	}))
	defer server.Close()

	client := NewJenkinsClient("test")
	client.BaseURL = server.URL

	jobs, err := client.Jobs(t.Context())
	if err != nil {
		t.Fatalf("jobs: %v", err)
	}
	if len(jobs) != 2 || jobs[0].Name != "api-build" || jobs[1].Status != "failed" {
		t.Fatalf("unexpected jobs: %#v", jobs)
	}
}

func TestJenkinsClientDiscoversNamespaceNamedHelmService(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })
	SetClient(fake.NewClientBuilder().WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-staging-ci",
			Namespace: "test-staging-ci",
			Labels: map[string]string{
				"app.kubernetes.io/name":      "jenkins",
				"app.kubernetes.io/component": "jenkins-controller",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 8080}},
		},
	}).Build())

	client := NewJenkinsClient("test-staging-ci")

	if client.BaseURL != "http://test-staging-ci.test-staging-ci.svc.cluster.local:8080" {
		t.Fatalf("base URL = %q", client.BaseURL)
	}
}

func TestJenkinsClientBuildsJob(t *testing.T) {
	var requestedPath string
	var requestedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crumbIssuer/api/json" {
			_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"abc123"}`))
			return
		}
		requestedPath = r.URL.EscapedPath()
		requestedToken = r.URL.Query().Get("token")
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.Header.Get("Jenkins-Crumb") != "abc123" {
			t.Fatalf("missing crumb header")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewJenkinsClient("test")
	client.BaseURL = server.URL

	if err := client.BuildJob(t.Context(), "folder/api build"); err != nil {
		t.Fatalf("build job: %v", err)
	}
	if requestedPath != "/job/folder/job/api%20build/buildWithParameters" {
		t.Fatalf("unexpected build path %s", requestedPath)
	}
	if requestedToken != "paap-source-build" {
		t.Fatalf("unexpected build token %q", requestedToken)
	}
}

func TestJenkinsClientCreatesPipelineJobWithCrumbAndBasicAuth(t *testing.T) {
	var createdPath string
	var createdXML string
	var crumbHeader string
	var crumbCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin123" {
			t.Fatalf("missing jenkins basic auth")
		}
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "crumb-session", Path: "/"})
			_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"abc123"}`))
		case "/job/shop-dev-orders-api-build/api/json":
			http.NotFound(w, r)
		case "/createItem":
			createdPath = r.URL.String()
			crumbHeader = r.Header.Get("Jenkins-Crumb")
			for _, cookie := range r.Cookies() {
				if cookie.Name == "JSESSIONID" {
					crumbCookie = cookie.Value
				}
			}
			body, _ := io.ReadAll(r.Body)
			createdXML = string(body)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewJenkinsClient("test")
	client.BaseURL = server.URL

	err := client.EnsurePipelineJob(t.Context(), JenkinsPipelineJobSpec{
		Name:       "shop-dev-orders-api-build",
		RepoURL:    "http://gitea/paap/shop-dev-components.git",
		Branch:     "main",
		ScriptPath: "components/orders-api/Jenkinsfile",
	})
	if err != nil {
		t.Fatalf("ensure pipeline job: %v", err)
	}
	if createdPath != "/createItem?name=shop-dev-orders-api-build" {
		t.Fatalf("unexpected create path %s", createdPath)
	}
	if crumbHeader != "abc123" {
		t.Fatalf("missing crumb header, got %q", crumbHeader)
	}
	if crumbCookie != "crumb-session" {
		t.Fatalf("missing crumb session cookie, got %q", crumbCookie)
	}
	for _, want := range []string{
		"<flow-definition",
		"<url>http://gitea/paap/shop-dev-components.git</url>",
		"<name>*/main</name>",
		"<scriptPath>components/orders-api/Jenkinsfile</scriptPath>",
		"<hudson.triggers.SCMTrigger>",
		"<authToken>paap-source-build</authToken>",
	} {
		if !strings.Contains(createdXML, want) {
			t.Fatalf("created job XML missing %q:\n%s", want, createdXML)
		}
	}
}

func TestJenkinsClientUpdatesExistingPipelineJob(t *testing.T) {
	var updatePath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"abc123"}`))
		case "/job/shop-dev-orders-api-build/api/json":
			w.WriteHeader(http.StatusOK)
		case "/job/shop-dev-orders-api-build/config.xml":
			updatePath = r.URL.Path
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected update method %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewJenkinsClient("test")
	client.BaseURL = server.URL

	err := client.EnsurePipelineJob(t.Context(), JenkinsPipelineJobSpec{
		Name:       "shop-dev-orders-api-build",
		RepoURL:    "http://gitea/paap/shop-dev-components.git",
		Branch:     "main",
		ScriptPath: "components/orders-api/Jenkinsfile",
	})
	if err != nil {
		t.Fatalf("ensure pipeline job: %v", err)
	}
	if updatePath != "/job/shop-dev-orders-api-build/config.xml" {
		t.Fatalf("expected config.xml update, got %s", updatePath)
	}
}
