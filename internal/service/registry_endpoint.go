package service

import (
	"fmt"
	"os"
	"strings"

	"paap/internal/model"
)

const (
	DefaultRegistryHostTemplate = "registry.{app}-{env}.paap.local:5000"
	RegistryHostTemplateEnv     = "PAAP_REGISTRY_HOST_TEMPLATE"
)

func RuntimeRegistryHost(app model.Application, env model.Environment, serviceType string) string {
	primaryNS := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	toolNS := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, serviceType)
	template := strings.TrimSpace(os.Getenv(RegistryHostTemplateEnv))
	if template == "" {
		template = DefaultRegistryHostTemplate
	}
	replacements := map[string]string{
		"{app}":              app.Identifier,
		"{env}":              env.Identifier,
		"{primaryNamespace}": primaryNS,
		"{toolNamespace}":    toolNS,
		"{service}":          serviceType,
	}
	host := template
	for token, value := range replacements {
		host = strings.ReplaceAll(host, token, value)
	}
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(host), "https://"), "http://")
}

func RuntimeRegistryImage(app model.Application, env model.Environment, serviceType, repository, tag string) string {
	host := RuntimeRegistryHost(app, env, serviceType)
	repository = strings.Trim(repository, "/")
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/%s:%s", host, repository, tag)
}
