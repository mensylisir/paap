package model

import (
	"strings"
	"testing"
)

func TestComponentConfigKeepsBindingsAndGeneratedObjects(t *testing.T) {
	cfg := ComponentConfig{
		Framework: " springboot ",
		ConfigMaps: []ComponentConfigMap{{
			Name: "orders-config",
			Data: map[string]string{" application.yml ": "spring: {}"},
		}},
		Secrets: []ComponentSecret{{
			Name: "orders-secret",
			Data: map[string]string{" POSTGRES_PASSWORD ": "secret"},
		}},
		Files: []ComponentConfigFile{{
			ConfigMapName: "orders-config",
			Key:           "application.yml",
			MountPath:     "/etc/paap/application.yml",
		}},
		Bindings: []ComponentBinding{{
			TargetKey:  "service:1",
			TargetName: "postgresql",
			TargetType: "postgresql",
			Role:       "database",
			Mode:       "springboot-file",
			Generated:  map[string]string{" POSTGRES_HOST ": "postgresql.ns.svc.cluster.local"},
		}},
		Dependencies: []string{"postgresql", "postgresql"},
	}

	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	parsed, err := ParseComponentConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if parsed.Framework != "springboot" {
		t.Fatalf("framework = %q", parsed.Framework)
	}
	if len(parsed.ConfigMaps) != 1 || parsed.ConfigMaps[0].Data["application.yml"] != "spring: {}" {
		t.Fatalf("configMaps not normalized: %#v", parsed.ConfigMaps)
	}
	if len(parsed.Secrets) != 1 || parsed.Secrets[0].Data["POSTGRES_PASSWORD"] != "secret" {
		t.Fatalf("secrets not normalized: %#v", parsed.Secrets)
	}
	if len(parsed.Files) != 1 || parsed.Files[0].Name == "" || parsed.Files[0].MountPath != "/etc/paap/application.yml" {
		t.Fatalf("files not normalized: %#v", parsed.Files)
	}
	if len(parsed.Bindings) != 1 || parsed.Bindings[0].Generated["POSTGRES_HOST"] == "" {
		t.Fatalf("bindings not normalized: %#v", parsed.Bindings)
	}
	if len(parsed.Dependencies) != 1 {
		t.Fatalf("dependencies not de-duplicated: %#v", parsed.Dependencies)
	}
}

func TestComponentConfigRejectsDuplicateGeneratedObjects(t *testing.T) {
	_, err := NormalizeComponentConfig(ComponentConfig{
		ConfigMaps: []ComponentConfigMap{
			{Name: "orders-config", Data: map[string]string{"a": "b"}},
			{Name: "orders-config", Data: map[string]string{"c": "d"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate configMap") {
		t.Fatalf("expected duplicate configMap error, got %v", err)
	}

	_, err = NormalizeComponentConfig(ComponentConfig{
		Files: []ComponentConfigFile{{
			Name:          "cfg",
			ConfigMapName: "orders-config",
			Key:           "application.yml",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "configMapName, key and mountPath") {
		t.Fatalf("expected incomplete config file error, got %v", err)
	}
}
