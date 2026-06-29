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
	BuildModule         string `gorm:"size:200" json:"buildModule,omitempty"`
	DockerfilePath      string `gorm:"size:200" json:"dockerfilePath,omitempty"`
	JenkinsJob          string `gorm:"size:200" json:"jenkinsJob,omitempty"`
	RegistryImage       string `gorm:"size:300" json:"registryImage,omitempty"`
	PipelineStatus      string `gorm:"size:30" json:"pipelineStatus,omitempty"`
	Config              string `gorm:"type:text" json:"config,omitempty"`
}

type ComponentConfig struct {
	Framework          string                      `json:"framework,omitempty"`
	ConfigTemplateID   uint                        `json:"configTemplateId,omitempty"`
	ConfigTemplateKey  string                      `json:"configTemplateKey,omitempty"`
	ConfigTemplateName string                      `json:"configTemplateName,omitempty"`
	ConfigTemplate     *ComponentConfigTemplateRef `json:"configTemplate,omitempty"`
	RegistryTarget     *ComponentRegistryTarget    `json:"registryTarget,omitempty"`
	ContainerPort      int32                       `json:"containerPort,omitempty"`
	ServicePort        int32                       `json:"servicePort,omitempty"`
	Command            []string                    `json:"command,omitempty"`
	Args               []string                    `json:"args,omitempty"`
	Env                []ComponentEnvVar           `json:"env,omitempty"`
	ConfigMaps         []ComponentConfigMap        `json:"configMaps,omitempty"`
	Secrets            []ComponentSecret           `json:"secrets,omitempty"`
	Files              []ComponentConfigFile       `json:"files,omitempty"`
	Bindings           []ComponentBinding          `json:"bindings,omitempty"`
	Dependencies       []string                    `json:"dependencies,omitempty"`
}

type ComponentConfigTemplateRef struct {
	ID   uint   `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

type ComponentRegistryTarget struct {
	Key          string `json:"key,omitempty"`
	Source       string `json:"source,omitempty"`
	Host         string `json:"host,omitempty"`
	ServiceID    uint   `json:"serviceId,omitempty"`
	CapabilityID uint   `json:"capabilityId,omitempty"`
	ServiceType  string `json:"serviceType,omitempty"`
	Name         string `json:"name,omitempty"`
}

type ComponentEnvVar struct {
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	SecretName    string `json:"secretName,omitempty"`
	SecretKey     string `json:"secretKey,omitempty"`
	ConfigMapName string `json:"configMapName,omitempty"`
	ConfigMapKey  string `json:"configMapKey,omitempty"`
}

type ComponentConfigMap struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type ComponentSecret struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type ComponentConfigFile struct {
	Name          string `json:"name"`
	ConfigMapName string `json:"configMapName"`
	Key           string `json:"key"`
	MountPath     string `json:"mountPath"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
}

type ComponentBinding struct {
	TargetKey  string            `json:"targetKey,omitempty"`
	TargetKind string            `json:"targetKind,omitempty"`
	TargetName string            `json:"targetName"`
	TargetType string            `json:"targetType,omitempty"`
	Role       string            `json:"role,omitempty"`
	Mode       string            `json:"mode,omitempty"`
	Confidence string            `json:"confidence,omitempty"`
	Source     string            `json:"source,omitempty"`
	Generated  map[string]string `json:"generated,omitempty"`
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
		Framework:          strings.TrimSpace(cfg.Framework),
		ConfigTemplateID:   cfg.ConfigTemplateID,
		ConfigTemplateKey:  strings.TrimSpace(cfg.ConfigTemplateKey),
		ConfigTemplateName: strings.TrimSpace(cfg.ConfigTemplateName),
		ContainerPort:      cfg.ContainerPort,
		ServicePort:        cfg.ServicePort,
		Command:            make([]string, 0, len(cfg.Command)),
		Args:               make([]string, 0, len(cfg.Args)),
		Env:                make([]ComponentEnvVar, 0, len(cfg.Env)),
		ConfigMaps:         make([]ComponentConfigMap, 0, len(cfg.ConfigMaps)),
		Secrets:            make([]ComponentSecret, 0, len(cfg.Secrets)),
		Files:              make([]ComponentConfigFile, 0, len(cfg.Files)),
		Bindings:           make([]ComponentBinding, 0, len(cfg.Bindings)),
		Dependencies:       make([]string, 0, len(cfg.Dependencies)),
	}
	if cfg.ConfigTemplate != nil {
		out.ConfigTemplate = &ComponentConfigTemplateRef{
			ID:   cfg.ConfigTemplate.ID,
			Key:  strings.TrimSpace(cfg.ConfigTemplate.Key),
			Name: strings.TrimSpace(cfg.ConfigTemplate.Name),
		}
		if out.ConfigTemplateID == 0 {
			out.ConfigTemplateID = out.ConfigTemplate.ID
		}
		if out.ConfigTemplateKey == "" {
			out.ConfigTemplateKey = out.ConfigTemplate.Key
		}
		if out.ConfigTemplateName == "" {
			out.ConfigTemplateName = out.ConfigTemplate.Name
		}
	}
	if cfg.RegistryTarget != nil {
		out.RegistryTarget = &ComponentRegistryTarget{
			Key:          strings.TrimSpace(cfg.RegistryTarget.Key),
			Source:       strings.TrimSpace(cfg.RegistryTarget.Source),
			Host:         strings.TrimSpace(cfg.RegistryTarget.Host),
			ServiceID:    cfg.RegistryTarget.ServiceID,
			CapabilityID: cfg.RegistryTarget.CapabilityID,
			ServiceType:  strings.TrimSpace(cfg.RegistryTarget.ServiceType),
			Name:         strings.TrimSpace(cfg.RegistryTarget.Name),
		}
		if out.RegistryTarget.Key == "" &&
			out.RegistryTarget.Source == "" &&
			out.RegistryTarget.Host == "" &&
			out.RegistryTarget.ServiceID == 0 &&
			out.RegistryTarget.CapabilityID == 0 &&
			out.RegistryTarget.ServiceType == "" &&
			out.RegistryTarget.Name == "" {
			out.RegistryTarget = nil
		}
	}
	if out.ConfigTemplateID > 0 || out.ConfigTemplateKey != "" || out.ConfigTemplateName != "" {
		out.ConfigTemplate = &ComponentConfigTemplateRef{
			ID:   out.ConfigTemplateID,
			Key:  out.ConfigTemplateKey,
			Name: out.ConfigTemplateName,
		}
	}
	if out.ContainerPort < 0 || out.ContainerPort > 65535 {
		return ComponentConfig{}, fmt.Errorf("containerPort must be between 0 and 65535")
	}
	if out.ServicePort < 0 || out.ServicePort > 65535 {
		return ComponentConfig{}, fmt.Errorf("servicePort must be between 0 and 65535")
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
	if err := normalizeComponentConfigMaps(cfg.ConfigMaps, &out); err != nil {
		return ComponentConfig{}, err
	}
	if err := normalizeComponentSecrets(cfg.Secrets, &out); err != nil {
		return ComponentConfig{}, err
	}
	seenFiles := map[string]struct{}{}
	for _, item := range cfg.Files {
		item.Name = strings.TrimSpace(item.Name)
		item.ConfigMapName = strings.TrimSpace(item.ConfigMapName)
		item.Key = strings.TrimSpace(item.Key)
		item.MountPath = strings.TrimSpace(item.MountPath)
		if item.Name == "" {
			item.Name = item.ConfigMapName + "-" + item.Key
		}
		if item.Name == "" && item.MountPath == "" {
			continue
		}
		if item.ConfigMapName == "" || item.Key == "" || item.MountPath == "" {
			return ComponentConfig{}, fmt.Errorf("config file %s configMapName, key and mountPath are required", valueOrDash(item.Name))
		}
		if _, exists := seenFiles[item.MountPath]; exists {
			return ComponentConfig{}, fmt.Errorf("duplicate config file mountPath %s", item.MountPath)
		}
		seenFiles[item.MountPath] = struct{}{}
		out.Files = append(out.Files, item)
	}
	seenBindings := map[string]struct{}{}
	for _, item := range cfg.Bindings {
		item.TargetKey = strings.TrimSpace(item.TargetKey)
		item.TargetKind = strings.TrimSpace(item.TargetKind)
		item.TargetName = strings.TrimSpace(item.TargetName)
		item.TargetType = strings.TrimSpace(item.TargetType)
		item.Role = strings.TrimSpace(item.Role)
		item.Mode = strings.TrimSpace(item.Mode)
		item.Confidence = strings.TrimSpace(item.Confidence)
		item.Source = strings.TrimSpace(item.Source)
		if item.TargetName == "" && item.TargetKey == "" {
			continue
		}
		generated := map[string]string{}
		for key, value := range item.Generated {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			generated[key] = strings.TrimSpace(value)
		}
		item.Generated = generated
		bindingKey := item.TargetKey + "|" + item.TargetName + "|" + item.Role
		if _, exists := seenBindings[bindingKey]; exists {
			continue
		}
		seenBindings[bindingKey] = struct{}{}
		out.Bindings = append(out.Bindings, item)
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

func ResolveComponentContainerPort(componentType string, cfg ComponentConfig) int32 {
	if cfg.ContainerPort > 0 {
		return cfg.ContainerPort
	}
	return DefaultComponentContainerPort(componentType)
}

func DefaultComponentContainerPort(componentType string) int32 {
	if strings.EqualFold(strings.TrimSpace(componentType), "frontend") {
		return 80
	}
	return 8080
}

func normalizeComponentConfigMaps(items []ComponentConfigMap, out *ComponentConfig) error {
	seen := map[string]struct{}{}
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if _, exists := seen[item.Name]; exists {
			return fmt.Errorf("duplicate configMap %s", item.Name)
		}
		seen[item.Name] = struct{}{}
		data := map[string]string{}
		for key, value := range item.Data {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			data[key] = value
		}
		if len(data) == 0 {
			return fmt.Errorf("configMap %s must contain data", item.Name)
		}
		item.Data = data
		out.ConfigMaps = append(out.ConfigMaps, item)
	}
	return nil
}

func normalizeComponentSecrets(items []ComponentSecret, out *ComponentConfig) error {
	seen := map[string]struct{}{}
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if _, exists := seen[item.Name]; exists {
			return fmt.Errorf("duplicate secret %s", item.Name)
		}
		seen[item.Name] = struct{}{}
		data := map[string]string{}
		for key, value := range item.Data {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			data[key] = value
		}
		if len(data) == 0 {
			return fmt.Errorf("secret %s must contain data", item.Name)
		}
		item.Data = data
		out.Secrets = append(out.Secrets, item)
	}
	return nil
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
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
