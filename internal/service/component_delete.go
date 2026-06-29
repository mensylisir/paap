package service

import (
	"context"
	"fmt"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
)

func DeleteComponent(ctx context.Context, db *gorm.DB, componentID uint) error {
	var comp model.Component
	if err := db.First(&comp, componentID).Error; err != nil {
		return ErrComponentNotFound
	}

	env, err := findEnvironment(db, comp.EnvironmentID)
	if err != nil {
		return err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return err
	}

	identifier := ComponentDeleteIdentifier(app, env, comp)
	argocdNamespace := componentArgoCDNamespace(db, env.ID, app, env)
	argocdApp := componentArgoCDApplicationName(app, env, comp, identifier)
	if err := k8s.DeleteArgoCDApplication(ctx, argocdNamespace, argocdApp); err != nil {
		return fmt.Errorf("ArgoCD Application delete failed: %w", err)
	}
	if err := k8s.DeleteComponentCR(ctx, app.Identifier, env.Identifier, identifier); err != nil {
		return fmt.Errorf("Component CR delete failed: %w", err)
	}
	generatedIdentifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	if err := k8s.DeleteComponentRuntimeResources(ctx, env.Namespace, identifier, generatedIdentifier, comp.Name, comp.GitPath); err != nil {
		return fmt.Errorf("Component runtime resources delete failed: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&comp).Error; err != nil {
			return err
		}
		return RemoveComponentFromCanvasState(tx, comp.EnvironmentID, comp.ID)
	})
}

func ComponentDeleteIdentifier(app model.Application, env model.Environment, comp model.Component) string {
	if identifier := componentIdentifierFromArgoCDApp(app, env, comp.ArgoCDApp); identifier != "" {
		return identifier
	}
	if identifier := componentIdentifierFromGitPath(comp.GitPath); identifier != "" {
		return identifier
	}
	return ComponentIdentifier(comp.Name, comp.Type, comp.ID)
}

func componentIdentifierFromArgoCDApp(app model.Application, env model.Environment, appName string) string {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return ""
	}
	prefix := strings.Trim(strings.TrimSpace(app.Identifier)+"-"+strings.TrimSpace(env.Identifier)+"-", "-")
	if prefix == "" || !strings.HasPrefix(appName, prefix) {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(appName, prefix), "-")
}

func componentIdentifierFromGitPath(gitPath string) string {
	gitPath = strings.Trim(strings.TrimSpace(gitPath), "/")
	if gitPath == "" {
		return ""
	}
	if strings.HasPrefix(gitPath, "components/") {
		gitPath = strings.TrimPrefix(gitPath, "components/")
	}
	if slash := strings.Index(gitPath, "/"); slash >= 0 {
		gitPath = gitPath[:slash]
	}
	return strings.Trim(gitPath, "-")
}

func componentArgoCDApplicationName(app model.Application, env model.Environment, comp model.Component, identifier string) string {
	if strings.TrimSpace(comp.ArgoCDApp) != "" {
		return strings.TrimSpace(comp.ArgoCDApp)
	}
	return fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier)
}

func componentArgoCDNamespace(db *gorm.DB, envID uint, app model.Application, env model.Environment) string {
	var inst model.ServiceInstallation
	if err := db.
		Where("environment_id = ? AND service_type = ?", envID, "deploy").
		First(&inst).Error; err == nil && strings.TrimSpace(inst.Namespace) != "" {
		return strings.TrimSpace(inst.Namespace)
	}
	return fmt.Sprintf("%s-%s-argocd", app.Identifier, env.Identifier)
}
