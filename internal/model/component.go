package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	paapv1 "paap/api/v1"

	"gorm.io/gorm"
)

// Component represents a business service component in an environment
type Component struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	EnvironmentID uint           `gorm:"not null;index" json:"environmentId"`
	Name          string         `gorm:"size:50;not null" json:"name"`
	Type          string         `gorm:"size:20;not null" json:"type"` // frontend, backend, database, middleware, custom
	Image         string         `gorm:"size:200" json:"image"`
	Version       string         `gorm:"size:50" json:"version"`
	Replicas      int            `gorm:"default:1" json:"replicas"`
	CPU           string         `gorm:"size:20;default:0.5核" json:"cpu"`
	Memory        string         `gorm:"size:20;default:512MB" json:"memory"`
	Status        string         `gorm:"size:20;default:stopped" json:"status"` // running, stopped, error
	ErrorMessage  string         `gorm:"type:text" json:"errorMessage"`
	GitRepoURL    string         `gorm:"size:500" json:"gitRepoUrl,omitempty"`
	GitPath       string         `gorm:"size:200" json:"gitPath,omitempty"`
	ArgoCDApp     string         `gorm:"size:100" json:"argocdApp,omitempty"`
	DeliveryMode  string         `gorm:"size:20;default:image" json:"deliveryMode,omitempty"`
	SourceRepoURL string         `gorm:"size:500" json:"sourceRepoUrl,omitempty"`
	// SourceMirrorRepoURL is the environment-local Gitea repository used by Jenkins/kpack.
	SourceMirrorRepoURL string `gorm:"size:500" json:"sourceMirrorRepoUrl,omitempty"`
	SourceBranch        string `gorm:"size:100" json:"sourceBranch,omitempty"`
	BuildContext        string `gorm:"size:200" json:"buildContext,omitempty"`
	DockerfilePath      string `gorm:"size:200" json:"dockerfilePath,omitempty"`
	JenkinsJob          string `gorm:"size:200" json:"jenkinsJob,omitempty"`
	RegistryImage       string `gorm:"size:300" json:"registryImage,omitempty"`
	PipelineStatus      string `gorm:"size:30" json:"pipelineStatus,omitempty"`
	Config              string `gorm:"type:text" json:"config,omitempty"`
}

type ComponentConfig struct {
	Command      []string          `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          []ComponentEnvVar `json:"env,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
}

type ComponentEnvVar struct {
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	SecretName    string `json:"secretName,omitempty"`
	SecretKey     string `json:"secretKey,omitempty"`
	ConfigMapName string `json:"configMapName,omitempty"`
	ConfigMapKey  string `json:"configMapKey,omitempty"`
}

func (cfg ComponentConfig) JSON() (string, error) {
	normalized, err := NormalizeComponentConfig(cfg)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseComponentConfig(raw string) (ComponentConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ComponentConfig{}, nil
	}
	var cfg ComponentConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ComponentConfig{}, err
	}
	return NormalizeComponentConfig(cfg)
}

func NormalizeComponentConfig(cfg ComponentConfig) (ComponentConfig, error) {
	out := ComponentConfig{
		Command:      make([]string, 0, len(cfg.Command)),
		Args:         make([]string, 0, len(cfg.Args)),
		Env:          make([]ComponentEnvVar, 0, len(cfg.Env)),
		Dependencies: make([]string, 0, len(cfg.Dependencies)),
	}
	for _, item := range cfg.Command {
		item = strings.TrimSpace(item)
		if item != "" {
			out.Command = append(out.Command, item)
		}
	}
	for _, item := range cfg.Args {
		item = strings.TrimSpace(item)
		if item != "" {
			out.Args = append(out.Args, item)
		}
	}
	seenEnv := map[string]struct{}{}
	for _, item := range cfg.Env {
		item.Name = strings.TrimSpace(item.Name)
		item.Value = strings.TrimSpace(item.Value)
		item.SecretName = strings.TrimSpace(item.SecretName)
		item.SecretKey = strings.TrimSpace(item.SecretKey)
		item.ConfigMapName = strings.TrimSpace(item.ConfigMapName)
		item.ConfigMapKey = strings.TrimSpace(item.ConfigMapKey)
		if item.Name == "" {
			return ComponentConfig{}, fmt.Errorf("env name is required")
		}
		if _, exists := seenEnv[item.Name]; exists {
			return ComponentConfig{}, fmt.Errorf("duplicate env name %s", item.Name)
		}
		seenEnv[item.Name] = struct{}{}
		usesSecret := item.SecretName != "" || item.SecretKey != ""
		usesConfigMap := item.ConfigMapName != "" || item.ConfigMapKey != ""
		if usesSecret && usesConfigMap {
			return ComponentConfig{}, fmt.Errorf("env %s cannot reference both secret and configmap", item.Name)
		}
		if usesSecret && (item.SecretName == "" || item.SecretKey == "") {
			return ComponentConfig{}, fmt.Errorf("env %s secretName and secretKey are required together", item.Name)
		}
		if usesConfigMap && (item.ConfigMapName == "" || item.ConfigMapKey == "") {
			return ComponentConfig{}, fmt.Errorf("env %s configMapName and configMapKey are required together", item.Name)
		}
		out.Env = append(out.Env, item)
	}
	seenDeps := map[string]struct{}{}
	for _, dep := range cfg.Dependencies {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, exists := seenDeps[dep]; exists {
			continue
		}
		seenDeps[dep] = struct{}{}
		out.Dependencies = append(out.Dependencies, dep)
	}
	return out, nil
}

func ComponentConfigFromEnvVars(envVars []paapv1.EnvVar) ComponentConfig {
	cfg := ComponentConfig{Env: make([]ComponentEnvVar, 0, len(envVars))}
	for _, env := range envVars {
		item := ComponentEnvVar{Name: env.Name, Value: env.Value}
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			item.SecretName = env.ValueFrom.SecretKeyRef.Name
			item.SecretKey = env.ValueFrom.SecretKeyRef.Key
		}
		if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
			item.ConfigMapName = env.ValueFrom.ConfigMapKeyRef.Name
			item.ConfigMapKey = env.ValueFrom.ConfigMapKeyRef.Key
		}
		cfg.Env = append(cfg.Env, item)
	}
	return cfg
}

func ComponentEnvVars(configJSON string) ([]paapv1.EnvVar, error) {
	cfg, err := ParseComponentConfig(configJSON)
	if err != nil {
		return nil, err
	}
	envVars := make([]paapv1.EnvVar, 0, len(cfg.Env))
	for _, item := range cfg.Env {
		env := paapv1.EnvVar{Name: item.Name, Value: item.Value}
		if item.SecretName != "" || item.SecretKey != "" {
			env.Value = ""
			env.ValueFrom = &paapv1.EnvVarSource{
				SecretKeyRef: &paapv1.SecretKeySelector{Name: item.SecretName, Key: item.SecretKey},
			}
		}
		if item.ConfigMapName != "" || item.ConfigMapKey != "" {
			env.Value = ""
			env.ValueFrom = &paapv1.EnvVarSource{
				ConfigMapKeyRef: &paapv1.ConfigMapKeySelector{Name: item.ConfigMapName, Key: item.ConfigMapKey},
			}
		}
		envVars = append(envVars, env)
	}
	return envVars, nil
}

// ServiceInstance represents a tool service instance in an application (deprecated, use ServiceInstallation)
type ServiceInstance struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	ApplicationID uint           `gorm:"not null;index" json:"applicationId"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	ServiceType   string         `gorm:"size:20;not null" json:"serviceType"`   // deploy, ci, monitor, log
	Status        string         `gorm:"size:20;default:pending" json:"status"` // pending, running, error
	Config        string         `gorm:"type:text" json:"config"`               // JSON config
}
