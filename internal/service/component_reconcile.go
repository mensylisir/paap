package service

import (
	"context"
	"fmt"
	"strings"

	"paap/internal/model"

	"gorm.io/gorm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileEnvironmentGitOps realigns component delivery state from PAAP's
// built-in flow engine and persists the resulting component metadata.
func ReconcileEnvironmentGitOps(ctx context.Context, db *gorm.DB, k8sClient client.Client, app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) (ToolWorkspace, []string) {
	primaryNS := app.Identifier + "-" + env.Identifier
	errs := make([]string, 0)

	for i := range components {
		comp := components[i]
		identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		result, err := runComponentDeliveryFlow(ctx, db, k8sClient, app, env, comp, identifier, primaryNS)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", comp.Name, err))
			continue
		}
		comp = applyComponentGitOpsResult(comp, result)
		if err := db.Save(&comp).Error; err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", comp.Name, err))
			continue
		}
		components[i] = comp
	}

	return BuildToolWorkspace(app, env, inst, components), errs
}

func runComponentDeliveryFlow(ctx context.Context, db *gorm.DB, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, namespace string) (ComponentGitOpsResult, error) {
	targets, err := loadComponentDeliveryTargets(db, env.ID)
	if err != nil {
		return ComponentGitOpsResult{}, err
	}
	if comp.DeliveryMode == "source" {
		return RunComponentSourceDeliveryFlow(ctx, k8sClient, app, env, comp, identifier, namespace, targets)
	}
	return RunComponentImageDeliveryFlow(ctx, k8sClient, app, env, comp, identifier, namespace, targets)
}

func applyComponentGitOpsResult(comp model.Component, result ComponentGitOpsResult) model.Component {
	comp.GitRepoURL = result.RepositoryURL
	if result.SourceMirrorURL != "" {
		comp.SourceMirrorRepoURL = result.SourceMirrorURL
	}
	comp.GitPath = result.RepositoryPath
	comp.ArgoCDApp = result.ArgoCDApplication
	if result.CIStatus != "" {
		comp.PipelineStatus = result.CIStatus
	}
	if result.CIWarning != "" {
		comp.ErrorMessage = result.CIWarning
	} else {
		comp.ErrorMessage = ""
	}
	if strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "failed") {
		comp.Status = "error"
	} else if comp.DeliveryMode == "source" && !strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built") {
		comp.Status = "building"
	} else {
		comp.Status = "syncing"
	}
	return comp
}
