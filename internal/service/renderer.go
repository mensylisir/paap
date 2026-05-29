package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// TemplateRenderer renders ServiceTemplate and EnvTemplate with runtime variables
type TemplateRenderer struct {
	funcMap template.FuncMap
}

// NewTemplateRenderer creates a new renderer with Sprig functions
func NewTemplateRenderer() *TemplateRenderer {
	funcMap := sprig.TxtFuncMap()
	// 自定义函数
	funcMap["json"] = func(v interface{}) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	funcMap["join"] = func(sep string, items []string) string {
		return strings.Join(items, sep)
	}
	funcMap["default"] = func(defaultVal interface{}, val interface{}) interface{} {
		if val == nil || val == "" {
			return defaultVal
		}
		return val
	}
	return &TemplateRenderer{funcMap: funcMap}
}

// TemplateVariables contains all runtime variables injected into templates
type TemplateVariables struct {
	// 应用信息
	AppID          uint   `json:"appId"`
	AppName        string `json:"appName"`
	AppIdentifier  string `json:"appIdentifier"`

	// 环境信息
	EnvID          uint     `json:"envId"`
	EnvName        string   `json:"envName"`
	EnvIdentifier  string   `json:"envIdentifier"`
	PrimaryNamespace string `json:"primaryNamespace"`
	ToolNamespace    string   `json:"toolNamespace"`
	Namespaces     []string `json:"namespaces"`

	// 新增/删除 namespace 时
	NewNamespace      string `json:"newNamespace,omitempty"`
	RemovedNamespace  string `json:"removedNamespace,omitempty"`

	// 服务实例信息
	ReleaseName            string `json:"releaseName"`
	ServiceAccountName     string `json:"serviceAccountName"`
	ServiceAccountNamespace string `json:"serviceAccountNamespace"`

	// 用户参数
	Parameters map[string]string `json:"parameters"`

	// RBAC 规则
	EnvRole RoleRules `json:"envRole"`
}

type RoleRules struct {
	Rules []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

// toMap converts TemplateVariables to a map for template rendering (supports camelCase keys)
func (v TemplateVariables) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"appId":                 v.AppID,
		"appName":               v.AppName,
		"appIdentifier":         v.AppIdentifier,
		"envId":                 v.EnvID,
		"envName":               v.EnvName,
		"envIdentifier":         v.EnvIdentifier,
		"primaryNamespace":      v.PrimaryNamespace,
		"toolNamespace":         v.ToolNamespace,
		"namespaces":            v.Namespaces,
		"releaseName":           v.ReleaseName,
		"serviceAccountName":    v.ServiceAccountName,
		"serviceAccountNamespace": v.ServiceAccountNamespace,
		"parameters":            v.Parameters,
		"envRole":               v.EnvRole,
	}
	if v.NewNamespace != "" {
		m["newNamespace"] = v.NewNamespace
	}
	if v.RemovedNamespace != "" {
		m["removedNamespace"] = v.RemovedNamespace
	}
	return m
}

// RenderString renders a Go template string with the given variables
func (r *TemplateRenderer) RenderString(tmplStr string, vars TemplateVariables) (string, error) {
	tmpl, err := template.New("render").Funcs(r.funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars.toMap()); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}

	return buf.String(), nil
}

// RenderServiceTemplate renders a ServiceTemplate with the given variables
// Returns the rendered YAML manifests
func (r *TemplateRenderer) RenderServiceTemplate(tmplStr string, vars TemplateVariables) (string, error) {
	return r.RenderString(tmplStr, vars)
}

// RenderEnvNsAdded renders the onEnvNsAdded template
func (r *TemplateRenderer) RenderEnvNsAdded(tmplStr string, vars TemplateVariables, newNS string) (string, error) {
	vars.NewNamespace = newNS
	return r.RenderString(tmplStr, vars)
}

// RenderEnvNsRemoved renders the onEnvNsRemoved template
func (r *TemplateRenderer) RenderEnvNsRemoved(tmplStr string, vars TemplateVariables, removedNS string) (string, error) {
	vars.RemovedNamespace = removedNS
	return r.RenderString(tmplStr, vars)
}

// BuildVariables builds template variables from application/environment context
func BuildVariables(appID uint, appName, appIdentifier string,
	envID uint, envName, envIdentifier string,
	primaryNS string, toolNS string, namespaces []string,
	saName, saNamespace string,
	parameters map[string]string,
	envRole RoleRules) TemplateVariables {

	return TemplateVariables{
		AppID:                 appID,
		AppName:               appName,
		AppIdentifier:         appIdentifier,
		EnvID:                 envID,
		EnvName:               envName,
		EnvIdentifier:         envIdentifier,
		PrimaryNamespace:      primaryNS,
		ToolNamespace:         toolNS,
		Namespaces:            namespaces,
		ReleaseName:           fmt.Sprintf("%s-%s", appIdentifier, envIdentifier),
		ServiceAccountName:    saName,
		ServiceAccountNamespace: saNamespace,
		Parameters:            parameters,
		EnvRole:               envRole,
	}
}
