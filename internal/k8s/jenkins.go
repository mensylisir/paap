package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type JenkinsClient struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

type JenkinsJob struct {
	Name      string        `json:"name"`
	URL       string        `json:"url"`
	Color     string        `json:"color"`
	LastBuild *JenkinsBuild `json:"lastBuild"`
	Status    string        `json:"status"`
}

type JenkinsBuild struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Result string `json:"result"`
}

type JenkinsPipelineJobSpec struct {
	Name       string
	RepoURL    string
	Branch     string
	ScriptPath string
	BuildToken string
}

func NewJenkinsClient(namespace string) *JenkinsClient {
	fallback := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", namespace, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	baseURL := discoverService(ctx, namespace, "jenkins", fallback)
	return &JenkinsClient{
		BaseURL:  baseURL,
		Username: "admin",
		Password: "admin123",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (j *JenkinsClient) Base() string {
	return j.BaseURL
}

func (j *JenkinsClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(j.BaseURL, "/")+"/login", nil)
	if err != nil {
		return err
	}
	j.addAuth(req)
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("jenkins health check failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("jenkins health check returned %d", res.StatusCode)
	}
	return nil
}

func (j *JenkinsClient) Jobs(ctx context.Context) ([]JenkinsJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(j.BaseURL, "/")+"/api/json?tree=jobs[name,url,color,lastBuild[number,url,result]]", nil)
	if err != nil {
		return nil, err
	}
	j.addAuth(req)
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins API returned %d", res.StatusCode)
	}
	var payload struct {
		Jobs []JenkinsJob `json:"jobs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	for i := range payload.Jobs {
		payload.Jobs[i].Status = jenkinsStatus(payload.Jobs[i].Color)
	}
	return payload.Jobs, nil
}

func (j *JenkinsClient) ConsoleText(ctx context.Context, jobName string) (string, error) {
	path := jenkinsJobPath(jobName) + "/lastBuild/consoleText"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(j.BaseURL, "/")+path, nil)
	if err != nil {
		return "", err
	}
	j.addAuth(req)
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("jenkins console request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jenkins console returned %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 64*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (j *JenkinsClient) BuildJob(ctx context.Context, jobName string) error {
	path := jenkinsJobPath(jobName) + "/buildWithParameters?token=paap-source-build"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(j.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	j.addAuth(req)
	if err := j.addCrumb(ctx, req); err != nil {
		return err
	}
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("jenkins build request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusOK {
		return fmt.Errorf("jenkins build returned %d", res.StatusCode)
	}
	return nil
}

func (j *JenkinsClient) EnsurePipelineJob(ctx context.Context, spec JenkinsPipelineJobSpec) error {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.RepoURL = strings.TrimSpace(spec.RepoURL)
	spec.Branch = strings.TrimSpace(spec.Branch)
	spec.ScriptPath = strings.TrimSpace(spec.ScriptPath)
	if spec.Name == "" {
		return fmt.Errorf("jenkins job name is required")
	}
	if spec.RepoURL == "" {
		return fmt.Errorf("jenkins pipeline repo URL is required")
	}
	if spec.Branch == "" {
		spec.Branch = "main"
	}
	if spec.ScriptPath == "" {
		spec.ScriptPath = "Jenkinsfile"
	}
	if spec.BuildToken == "" {
		spec.BuildToken = "paap-source-build"
	}

	exists, err := j.jobExists(ctx, spec.Name)
	if err != nil {
		return err
	}
	jobXML := buildPipelineJobXML(spec)
	var endpoint string
	if exists {
		endpoint = strings.TrimRight(j.BaseURL, "/") + jenkinsJobPath(spec.Name) + "/config.xml"
	} else {
		endpoint = strings.TrimRight(j.BaseURL, "/") + "/createItem?name=" + url.QueryEscape(spec.Name)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(jobXML))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	j.addAuth(req)
	if err := j.addCrumb(ctx, req); err != nil {
		return err
	}
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("jenkins pipeline job sync failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("jenkins pipeline job sync returned %d: %s", res.StatusCode, string(body))
	}
	return nil
}

func (j *JenkinsClient) jobExists(ctx context.Context, jobName string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(j.BaseURL, "/")+jenkinsJobPath(jobName)+"/api/json", nil)
	if err != nil {
		return false, err
	}
	j.addAuth(req)
	res, err := j.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("jenkins job lookup failed: %w", err)
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		body, _ := io.ReadAll(res.Body)
		return false, fmt.Errorf("jenkins job lookup returned %d: %s", res.StatusCode, string(body))
	}
}

func (j *JenkinsClient) addAuth(req *http.Request) {
	if j.Username != "" || j.Password != "" {
		req.SetBasicAuth(j.Username, j.Password)
	}
}

func (j *JenkinsClient) addCrumb(ctx context.Context, req *http.Request) error {
	crumbReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(j.BaseURL, "/")+"/crumbIssuer/api/json", nil)
	if err != nil {
		return err
	}
	j.addAuth(crumbReq)
	res, err := j.HTTPClient.Do(crumbReq)
	if err != nil {
		return fmt.Errorf("jenkins crumb request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden {
		return nil
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("jenkins crumb request returned %d: %s", res.StatusCode, string(body))
	}
	var payload struct {
		CrumbRequestField string `json:"crumbRequestField"`
		Crumb             string `json:"crumb"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.CrumbRequestField != "" && payload.Crumb != "" {
		req.Header.Set(payload.CrumbRequestField, payload.Crumb)
	}
	for _, cookie := range res.Cookies() {
		req.AddCookie(cookie)
	}
	return nil
}

func buildPipelineJobXML(spec JenkinsPipelineJobSpec) string {
	branch := strings.TrimSpace(spec.Branch)
	if branch == "" {
		branch = "main"
	}
	scriptPath := strings.TrimSpace(spec.ScriptPath)
	if scriptPath == "" {
		scriptPath = "Jenkinsfile"
	}
	return fmt.Sprintf(`<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <actions/>
  <description>PAAP managed pipeline for %s</description>
  <keepDependencies>false</keepDependencies>
  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions/>
    </hudson.model.ParametersDefinitionProperty>
  </properties>
  <authToken>%s</authToken>
  <triggers>
    <hudson.triggers.SCMTrigger>
      <spec>H/2 * * * *</spec>
      <ignorePostCommitHooks>false</ignorePostCommitHooks>
    </hudson.triggers.SCMTrigger>
  </triggers>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsScmFlowDefinition" plugin="workflow-cps">
    <scm class="hudson.plugins.git.GitSCM" plugin="git">
      <configVersion>2</configVersion>
      <userRemoteConfigs>
        <hudson.plugins.git.UserRemoteConfig>
          <url>%s</url>
        </hudson.plugins.git.UserRemoteConfig>
      </userRemoteConfigs>
      <branches>
        <hudson.plugins.git.BranchSpec>
          <name>*/%s</name>
        </hudson.plugins.git.BranchSpec>
      </branches>
      <doGenerateSubmoduleConfigurations>false</doGenerateSubmoduleConfigurations>
      <submoduleCfg class="empty-list"/>
      <extensions/>
    </scm>
    <scriptPath>%s</scriptPath>
    <lightweight>true</lightweight>
  </definition>
  <disabled>false</disabled>
</flow-definition>
`, html.EscapeString(spec.Name), html.EscapeString(spec.BuildToken), html.EscapeString(spec.RepoURL), html.EscapeString(branch), html.EscapeString(scriptPath))
}

func jenkinsJobPath(jobName string) string {
	return JenkinsJobPath(jobName)
}

func JenkinsJobPath(jobName string) string {
	segments := strings.Split(strings.Trim(jobName, "/"), "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		escaped = append(escaped, "job", url.PathEscape(segment))
	}
	if len(escaped) == 0 {
		return "/job"
	}
	return "/" + strings.Join(escaped, "/")
}

func jenkinsStatus(color string) string {
	switch {
	case strings.HasPrefix(color, "blue"):
		return "success"
	case strings.HasPrefix(color, "red"):
		return "failed"
	case strings.HasPrefix(color, "yellow"):
		return "unstable"
	case strings.Contains(color, "anime"):
		return "running"
	case color == "disabled":
		return "disabled"
	default:
		return "unknown"
	}
}
