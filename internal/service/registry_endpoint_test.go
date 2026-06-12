package service

import (
	"testing"

	"paap/internal/model"
)

func TestRuntimeRegistryHostDefaultsToEnvironmentScopedHost(t *testing.T) {
	host := RuntimeRegistryHost(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		"registry",
	)

	if host != "registry.shop-dev.paap.local:5000" {
		t.Fatalf("host = %q", host)
	}
}

func TestRuntimeRegistryHostUsesConfiguredTemplate(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "harbor-{app}-{env}.corp.example.com")

	host := RuntimeRegistryHost(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "prod"},
		"harbor",
	)

	if host != "harbor-shop-prod.corp.example.com" {
		t.Fatalf("host = %q", host)
	}
}

func TestRuntimeRegistryImageUsesRuntimeHost(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{primaryNamespace}.corp.example.com:5443")

	image := RuntimeRegistryImage(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "staging"},
		"registry",
		"billing-staging/api",
		"42",
	)

	if image != "registry.billing-staging.corp.example.com:5443/billing-staging/api:42" {
		t.Fatalf("image = %q", image)
	}
}
