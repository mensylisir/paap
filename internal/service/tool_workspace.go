package service

import (
	"fmt"
	"strings"

	"paap/internal/model"
)

type ToolWorkspace struct {
	Kind        string                  `json:"kind"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Actions     []ToolWorkspaceAction   `json:"actions"`
	Resources   []ToolWorkspaceResource `json:"resources"`
	Config      []ToolWorkspaceConfig   `json:"config"`
}

type ToolWorkspaceAction struct {
	Key         string                     `json:"key"`
	Label       string                     `json:"label"`
	Description string                     `json:"description"`
	Tone        string                     `json:"tone,omitempty"`
	Target      string                     `json:"target,omitempty"`
	Fields      []ToolWorkspaceActionField `json:"fields,omitempty"`
}

type ToolWorkspaceActionField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Default     string `json:"default,omitempty"`
}

type ToolWorkspaceResource struct {
	Name        string                  `json:"name"`
	Type        string                  `json:"type"`
	Status      string                  `json:"status"`
	Description string                  `json:"description"`
	ExternalURL string                  `json:"externalUrl,omitempty"`
	Actions     []ToolWorkspaceAction   `json:"actions,omitempty"`
	Annotations map[string]interface{}  `json:"annotations,omitempty"`
	Children    []ToolWorkspaceResource `json:"children,omitempty"`
}

type ToolWorkspaceConfig struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func BuildToolWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	switch inst.ServiceType {
	case "git":
		return buildRepositoryWorkspace(app, env, inst, components)
	case "deploy":
		return buildGitOpsWorkspace(app, env, inst, components)
	case "monitor", "log":
		return buildObservabilityWorkspace(app, env, inst, components)
	case "ci":
		return buildPipelineWorkspace(app, env, inst, components)
	case "registry", "harbor":
		return buildRegistryWorkspace(app, env, inst, components)
	case "mysql", "postgresql", "mongodb", "redis", "rabbitmq", "kafka", "minio":
		return buildDataWorkspace(inst)
	default:
		return buildGenericWorkspace(inst)
	}
}

func baseWorkspaceConfig(inst model.ServiceInstallation) []ToolWorkspaceConfig {
	config := []ToolWorkspaceConfig{
		{Label: "命名空间", Value: valueOrDash(inst.Namespace)},
		{Label: "Release", Value: valueOrDash(inst.ReleaseName)},
	}
	if entry := proxyURL(inst, ""); entry != "" {
		config = append(config, ToolWorkspaceConfig{Label: "平台代理入口", Value: entry})
	}
	config = append(config, ToolWorkspaceConfig{Label: "集群内地址", Value: accessURL(inst.Namespace, inst.ServiceType)})
	return config
}

func buildRepositoryWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	repoName := fmt.Sprintf("%s-%s-components", app.Identifier, env.Identifier)
	resources := make([]ToolWorkspaceResource, 0, len(components))
	gitopsRepos := map[string]int{}
	for _, comp := range components {
		identifier := componentIdentifierForWorkspace(comp)
		path := comp.GitPath
		if path == "" {
			path = "components/" + identifier
		}
		if comp.SourceMirrorRepoURL != "" {
			resources = append(resources, ToolWorkspaceResource{
				Name:        fmt.Sprintf("%s-%s-%s-source", app.Identifier, env.Identifier, identifier),
				Type:        "Repository",
				Status:      statusReadyOrPending(inst.Status),
				Description: fmt.Sprintf("组件 %s 的环境内源码镜像仓，Jenkins/kpack 从该仓构建。外部来源：%s", comp.Name, valueOrDefault(comp.SourceRepoURL, "-")),
				ExternalURL: strings.TrimSuffix(comp.SourceMirrorRepoURL, ".git"),
				Annotations: map[string]interface{}{
					"branch":         componentBranch(comp),
					"private":        false,
					"language":       componentLanguage(comp),
					"cloneURL":       comp.SourceMirrorRepoURL,
					"sourceRepoURL":  valueOrDefault(comp.SourceRepoURL, "-"),
					"repositoryRole": "source",
				},
				Actions: []ToolWorkspaceAction{
					{Key: "reconcile_gitops", Label: "同步源码镜像", Description: "重新确认源码镜像仓和交付配置。", Target: identifier},
				},
			})
		}
		cloneURL := valueOrDefault(comp.GitRepoURL, fmt.Sprintf("http://gitea/paap/%s.git", repoName))
		externalURL := strings.TrimSuffix(cloneURL, ".git")
		key := externalURL
		if key == "" {
			key = repoName
		}
		if idx, ok := gitopsRepos[key]; ok {
			resources[idx] = appendRepositoryComponent(resources[idx], comp, identifier, path)
			continue
		}
		gitopsRepos[key] = len(resources)
		resources = append(resources, appendRepositoryComponent(ToolWorkspaceResource{
			Name:        repositoryDisplayName(externalURL, repoName),
			Type:        "Repository",
			Status:      statusReadyOrPending(inst.Status),
			Description: fmt.Sprintf("默认分支 %s，PAAP 交付仓根目录；组件清单位于 components/*。", componentBranch(comp)),
			ExternalURL: externalURL,
			Annotations: map[string]interface{}{
				"branch":         componentBranch(comp),
				"private":        true,
				"language":       "Container/Kubernetes",
				"cloneURL":       cloneURL,
				"repositoryRole": "gitops",
			},
			Actions: []ToolWorkspaceAction{
				{Key: "reconcile_gitops", Label: "修复仓库", Description: "重新生成该仓库内所有组件的交付内容。"},
			},
		}, comp, identifier, path))
	}
	return ToolWorkspace{
		Kind:        "repository",
		Title:       "代码仓库",
		Description: "维护当前环境组件对应的 Gitea 仓库、默认分支和部署清单。",
		Actions: []ToolWorkspaceAction{
			{Key: "create_gitea_repository", Label: "创建仓库", Description: "在当前 Gitea 中创建一个新的代码仓库。", Tone: "primary", Fields: []ToolWorkspaceActionField{
				{Name: "name", Label: "仓库名称", Required: true, Placeholder: repoName},
				{Name: "description", Label: "描述", Placeholder: "组件源码或交付清单仓库"},
				{Name: "private", Label: "私有仓库", Type: "checkbox", Default: "true"},
			}},
			{Key: "add_gitea_user_key", Label: "添加 SSH 公钥", Description: "给 Gitea 用户添加 SSH 公钥，便于从本机 push 代码。", Fields: []ToolWorkspaceActionField{
				{Name: "title", Label: "名称", Required: true, Placeholder: "mensyli1-laptop"},
				{Name: "key", Label: "SSH 公钥", Type: "textarea", Required: true, Placeholder: "ssh-ed25519 AAAA..."},
			}},
			{Key: "add_gitea_deploy_key", Label: "添加 Deploy Key", Description: "给指定仓库添加部署公钥。", Fields: []ToolWorkspaceActionField{
				{Name: "repository", Label: "仓库", Required: true, Placeholder: repoName},
				{Name: "title", Label: "名称", Required: true, Placeholder: "paap-deploy-key"},
				{Name: "key", Label: "SSH 公钥", Type: "textarea", Required: true, Placeholder: "ssh-ed25519 AAAA..."},
				{Name: "readOnly", Label: "只读", Type: "checkbox", Default: "true"},
			}},
			{Key: "reconcile_gitops", Label: "同步代码仓", Description: "为所有组件同步 Gitea 仓库内容并刷新部署清单。", Tone: "primary"},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取工作区资源。"},
		},
		Resources: resources,
		Config: append(baseWorkspaceConfig(inst),
			ToolWorkspaceConfig{Label: "仓库命名", Value: repoName},
			ToolWorkspaceConfig{Label: "默认分支", Value: "main"},
		),
	}
}

func appendRepositoryComponent(repo ToolWorkspaceResource, comp model.Component, identifier, path string) ToolWorkspaceResource {
	if repo.Annotations == nil {
		repo.Annotations = map[string]interface{}{}
	}
	componentNames := stringSliceAnnotation(repo.Annotations, "components")
	componentPaths := stringSliceAnnotation(repo.Annotations, "componentPaths")
	componentNames = append(componentNames, valueOrDefault(comp.Name, identifier))
	componentPaths = append(componentPaths, path)
	repo.Annotations["components"] = componentNames
	repo.Annotations["componentPaths"] = componentPaths
	return repo
}

func stringSliceAnnotation(annotations map[string]interface{}, key string) []string {
	items, _ := annotations[key].([]string)
	out := make([]string, 0, len(items)+1)
	out = append(out, items...)
	return out
}

func repositoryDisplayName(externalURL, fallback string) string {
	externalURL = strings.TrimRight(strings.TrimSpace(externalURL), "/")
	if externalURL == "" {
		return fallback
	}
	parts := strings.Split(externalURL, "/")
	name := strings.TrimSpace(parts[len(parts)-1])
	if name == "" {
		return fallback
	}
	return name
}

func buildGitOpsWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	resources := make([]ToolWorkspaceResource, 0, len(components))
	for _, comp := range components {
		name := comp.ArgoCDApp
		if name == "" {
			name = fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, componentIdentifierForWorkspace(comp))
		}
		resources = append(resources, ToolWorkspaceResource{
			Name:        name,
			Type:        "Application",
			Status:      argoStatusFromComponent(comp),
			Description: fmt.Sprintf("组件 %s 的 ArgoCD Application。", comp.Name),
			Annotations: map[string]interface{}{
				"repoURL":     valueOrDefault(comp.GitRepoURL, fmt.Sprintf("http://gitea/paap/%s-%s-components.git", app.Identifier, env.Identifier)),
				"path":        valueOrDefault(comp.GitPath, "components/"+componentIdentifierForWorkspace(comp)),
				"syncStatus":  argoStatusFromComponent(comp),
				"health":      argoHealthFromComponent(comp),
				"namespace":   fmt.Sprintf("%s-%s", app.Identifier, env.Identifier),
				"server":      "https://kubernetes.default.svc",
				"revision":    valueOrDefault(comp.Version, "HEAD"),
				"destination": fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, componentIdentifierForWorkspace(comp)),
			},
			Actions: []ToolWorkspaceAction{
				{Key: "sync_argocd_application", Label: "同步", Description: "触发该 ArgoCD Application 同步。", Target: name},
				{Key: "delete_argocd_application", Label: "删除", Description: "删除该 ArgoCD Application。", Tone: "danger", Target: name},
			},
		})
	}
	return ToolWorkspace{
		Kind:        "gitops",
		Title:       "ArgoCD Applications",
		Description: "查看当前环境组件对应的 ArgoCD Application，并支持自动对齐或手动编辑。",
		Actions: []ToolWorkspaceAction{
			{Key: "reconcile_gitops", Label: "同步所有组件 Application", Description: "按 PAAP 组件批量创建缺失的 Application，并修复已存在 Application 的仓库、路径和同步策略。", Tone: "primary"},
			{Key: "apply_argocd_application", Label: "新建/编辑 Application", Description: "填写仓库、路径和目标命名空间；Project 自动绑定当前应用和环境，不允许手动覆盖。", Fields: argoCDApplicationFields()},
			{Key: "apply_argocd_applicationset", Label: "新建/编辑 ApplicationSet", Description: "按 Git 目录生成 Application；Project 自动绑定当前应用和环境，不允许手动覆盖。", Fields: argoCDApplicationSetFields()},
			{Key: "refresh", Label: "刷新同步状态", Description: "重新读取工作区资源。"},
		},
		Resources: resources,
		Config: append(baseWorkspaceConfig(inst),
			ToolWorkspaceConfig{Label: "Application 命名", Value: fmt.Sprintf("%s-%s-{component}", app.Identifier, env.Identifier)},
			ToolWorkspaceConfig{Label: "同步策略", Value: "Automated prune/selfHeal"},
		),
	}
}

func buildObservabilityWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	prefix := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	if inst.ServiceType == "monitor" {
		return ToolWorkspace{
			Kind:        "observability",
			Title:       "监控面板",
			Description: "查看当前环境的指标、日志和告警覆盖情况。",
			Actions: []ToolWorkspaceAction{
				{Key: "check_grafana_health", Label: "检查 Grafana", Description: "调用 Grafana API 检查服务是否可用。", Tone: "primary"},
				{Key: "list_grafana_dashboards", Label: "读取大盘", Description: "读取 Grafana 中已存在的 Dashboard。"},
				{Key: "list_prometheus_targets", Label: "监控目标", Description: "读取 Prometheus 当前抓取目标。"},
				{Key: "list_prometheus_alerts", Label: "告警", Description: "读取 Prometheus 当前告警。"},
				{Key: "list_prometheus_rules", Label: "规则", Description: "读取 Prometheus 规则组。"},
				{Key: "provision_grafana_dashboard", Label: "导入默认大盘", Description: "向 Grafana 导入 PAAP 环境总览 Dashboard。"},
				{Key: "refresh", Label: "刷新资源", Description: "重新读取监控资源。"},
			},
			Resources: append(monitorSubjectResources(app, env, components), []ToolWorkspaceResource{
				{Name: prefix + "-overview", Type: "Dashboard", Status: statusReadyOrPending(inst.Status), Description: "环境级服务总览、资源用量和错误率。", ExternalURL: proxyURL(inst, "/d/paap-"+prefix+"-overview"), Annotations: map[string]interface{}{"dashboardUid": "paap-" + prefix + "-overview", "subjectKind": "environment"}},
				{Name: prefix + "-pods", Type: "Dashboard", Status: statusReadyOrPending(inst.Status), Description: "组件 Pod CPU、内存、重启和网络。", ExternalURL: proxyURL(inst, "/d/paap-pod-workload"), Annotations: map[string]interface{}{"dashboardUid": "paap-pod-workload", "subjectKind": "component"}},
				{Name: prefix + "-tools", Type: "Dashboard", Status: statusReadyOrPending(inst.Status), Description: "平台工具 Kubernetes 工作负载指标。", ExternalURL: proxyURL(inst, "/d/paap-tool-workload"), Annotations: map[string]interface{}{"dashboardUid": "paap-tool-workload", "subjectKind": "tool"}},
				{Name: prefix + "-middleware", Type: "Dashboard", Status: statusReadyOrPending(inst.Status), Description: "数据库和中间件工作负载指标。", ExternalURL: proxyURL(inst, "/d/paap-middleware-workload"), Annotations: map[string]interface{}{"dashboardUid": "paap-middleware-workload", "subjectKind": "middleware"}},
			}...),
			Config: append(baseWorkspaceConfig(inst),
				ToolWorkspaceConfig{Label: "指标范围", Value: prefix + "-*"},
				ToolWorkspaceConfig{Label: "默认数据源", Value: "Prometheus / Loki"},
			),
		}
	}
	return ToolWorkspace{
		Kind:        "observability",
		Title:       "日志查询",
		Description: "查看当前环境的日志覆盖情况。",
		Actions: []ToolWorkspaceAction{
			{Key: "check_loki_health", Label: "检查 Loki", Description: "调用 Loki ready API 检查日志服务是否可用。", Tone: "primary"},
			{Key: "query_loki_streams", Label: "读取日志流", Description: "读取当前环境的 Loki labels 和日志流。"},
			{Key: "query_loki_logs", Label: "读取最近日志", Description: "读取当前环境的最新日志样本。"},
			{Key: "refresh", Label: "刷新日志资源", Description: "重新读取日志资源。"},
		},
		Resources: monitorSubjectResources(app, env, components),
		Config: append(baseWorkspaceConfig(inst),
			ToolWorkspaceConfig{Label: "日志范围", Value: prefix + "-*"},
			ToolWorkspaceConfig{Label: "默认数据源", Value: "Loki"},
		),
	}
}

func buildPipelineWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	resources := make([]ToolWorkspaceResource, 0, len(components))
	for _, comp := range components {
		jobName := valueOrDefault(comp.JenkinsJob, fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, componentIdentifierForWorkspace(comp)))
		resources = append(resources, ToolWorkspaceResource{
			Name:        jobName,
			Type:        "Job",
			Status:      jenkinsStatusFromComponent(comp, inst),
			Description: fmt.Sprintf("构建 %s 镜像并触发部署。", comp.Name),
			ExternalURL: proxyURL(inst, "/job/"+jobName+"/"),
			Annotations: map[string]interface{}{
				"color":     jenkinsColorFromComponent(comp, inst),
				"component": comp.Name,
				"branch":    componentBranch(comp),
				"image":     comp.RegistryImage,
			},
			Actions: []ToolWorkspaceAction{
				{Key: "trigger_jenkins_build", Label: "触发", Description: "触发该流水线构建。", Target: jobName},
			},
		})
	}
	return ToolWorkspace{
		Kind:        "pipeline",
		Title:       "CI 流水线",
		Description: "管理组件构建、测试、镜像推送和部署触发流程。",
		Actions: []ToolWorkspaceAction{
			{Key: "check_jenkins_health", Label: "检查 Jenkins", Description: "调用 Jenkins HTTP 入口检查服务是否可用。", Tone: "primary"},
			{Key: "trigger_jenkins_build", Label: "触发构建", Description: "触发第一个 Jenkins Job 构建。"},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取流水线资源。"},
		},
		Resources: resources,
		Config:    append(baseWorkspaceConfig(inst), ToolWorkspaceConfig{Label: "触发方式", Value: "代码推送 / 手动执行"}),
	}
}

func monitorSubjectResources(app model.Application, env model.Environment, components []model.Component) []ToolWorkspaceResource {
	prefix := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	resources := make([]ToolWorkspaceResource, 0, len(components)+1)
	resources = append(resources, ToolWorkspaceResource{
		Name:        prefix,
		Type:        "Monitor Subject",
		Status:      "Ready",
		Description: "当前环境全部组件、工具、数据库和中间件的聚合监控视图。",
		Annotations: map[string]interface{}{
			"subjectKind":   "environment",
			"namespace":     prefix,
			"selector":      prefix,
			"dashboardUid":  "paap-" + prefix + "-overview",
			"dashboardPath": "/d/paap-" + prefix + "-overview",
			"logQuery":      fmt.Sprintf(`{namespace=~"%s.*"}`, prefix),
		},
	})
	for _, comp := range components {
		identifier := componentIdentifierForWorkspace(comp)
		name := identifier
		if strings.TrimSpace(comp.Name) != "" {
			name = comp.Name
		}
		resources = append(resources, ToolWorkspaceResource{
			Name:        name,
			Type:        "Monitor Subject",
			Status:      statusReadyOrPending(comp.Status),
			Description: fmt.Sprintf("组件 %s 的指标、采集目标和告警。", name),
			Annotations: map[string]interface{}{
				"subjectKind":   "component",
				"component":     name,
				"namespace":     prefix,
				"selector":      identifier,
				"dashboardUid":  "paap-pod-workload",
				"dashboardPath": "/d/paap-pod-workload",
				"logQuery":      fmt.Sprintf(`{namespace="%s", pod=~"%s.*"}`, prefix, identifier),
			},
		})
	}
	return resources
}

func buildRegistryWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) ToolWorkspace {
	prefix := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	runtimeHost := RuntimeRegistryHost(app, env, inst.ServiceType)
	resources := make([]ToolWorkspaceResource, 0, len(components)+1)
	registryTagActions := registryWorkspaceTagActions(inst.ServiceType, "")
	for _, comp := range components {
		identifier := componentIdentifierForWorkspace(comp)
		tag := componentImageTag(comp)
		if !shouldExposeRegistryRepository(comp, tag) {
			continue
		}
		repoName := fmt.Sprintf("%s/%s", prefix, identifier)
		image := componentRegistryImageForWorkspace(app, env, inst, comp, repoName, tag)
		externalURL := registryRepositoryURL(inst, repoName, inst.ServiceType)
		resourceType := "Image Repository"
		status := statusReadyOrPending(inst.Status)
		description := fmt.Sprintf("组件 %s 的镜像推送目标。Tags: %s", comp.Name, tag)
		if !isEnvironmentRegistryImage(image, runtimeHost) {
			repoName = image
			resourceType = "External Image"
			status = "External"
			description = fmt.Sprintf("组件 %s 使用外部镜像，不在当前环境 Registry 中。", comp.Name)
			externalURL = ""
		}
		var resourceActions []ToolWorkspaceAction
		if resourceType == "Image Repository" {
			resourceActions = registryWorkspaceTagActions(inst.ServiceType, repoName)
		}
		resources = append(resources, ToolWorkspaceResource{
			Name:        repoName,
			Type:        resourceType,
			Status:      status,
			Description: description,
			ExternalURL: externalURL,
			Actions:     resourceActions,
			Annotations: map[string]interface{}{
				"project": prefix,
				"tags":    []string{tag},
				"image":   image,
			},
		})
	}
	resources = append(resources, runtimeRegistryTrustResource(inst, runtimeHost))
	healthLabel := "检查 Registry"
	healthDescription := "调用 Docker Registry HTTP API 检查服务是否可用。"
	if inst.ServiceType == "harbor" {
		healthLabel = "检查 Harbor"
		healthDescription = "调用 Harbor health API 检查服务是否可用。"
	}
	actions := []ToolWorkspaceAction{
		{Key: "check_registry_health", Label: healthLabel, Description: healthDescription, Tone: "primary"},
		{Key: "refresh", Label: "刷新资源", Description: "重新读取镜像仓库资源。"},
	}
	if len(registryTagActions) > 0 {
		actions = append([]ToolWorkspaceAction{actions[0]}, append(registryTagActions, actions[1])...)
	}
	return ToolWorkspace{
		Kind:        "registry",
		Title:       "镜像仓库",
		Description: "查看组件镜像仓库、标签和推送目标。",
		Actions:     actions,
		Resources:   resources,
		Config:      append(baseWorkspaceConfig(inst), ToolWorkspaceConfig{Label: "镜像命名", Value: prefix + "/{component}:{version}"}),
	}
}

func registryWorkspaceTagActions(serviceType, target string) []ToolWorkspaceAction {
	if serviceType != "registry" {
		return nil
	}
	fields := []ToolWorkspaceActionField{
		{Name: "tag", Label: "Tag", Required: true, Placeholder: "v1.0.0"},
	}
	if strings.TrimSpace(target) == "" {
		fields = append([]ToolWorkspaceActionField{{Name: "repository", Label: "Repository", Required: true, Placeholder: "app/frontend"}}, fields...)
	}
	return []ToolWorkspaceAction{{
		Key:         "delete_registry_tag",
		Label:       "删除 Tag",
		Description: "按 repository 和 tag 查询 manifest digest 并从 Docker Registry 删除；需要 Registry 已启用 deleteEnabled。",
		Tone:        "danger",
		Target:      target,
		Fields:      fields,
	}}
}

func buildDataWorkspace(inst model.ServiceInstallation) ToolWorkspace {
	actions := []ToolWorkspaceAction{{Key: "refresh", Label: "刷新资源", Description: "重新读取运行资源。", Tone: "primary"}}
	if inst.ServiceType == "mysql" || inst.ServiceType == "postgresql" {
		actions = []ToolWorkspaceAction{
			{Key: "check_database_connection", Label: "测试连接", Description: "使用 Kubernetes Secret 中的凭据测试数据库连接。", Tone: "primary"},
			{Key: "list_databases", Label: "查看数据库", Description: "读取数据库列表。"},
			{Key: "create_database", Label: "创建数据库", Description: "创建一个数据库。", Fields: []ToolWorkspaceActionField{{Name: "database", Label: "数据库名", Required: true, Placeholder: "appdb"}}},
			{Key: "drop_database", Label: "删除数据库", Description: "删除一个数据库。", Tone: "danger", Fields: []ToolWorkspaceActionField{{Name: "database", Label: "数据库名", Required: true, Placeholder: "appdb"}}},
			{Key: "create_database_backup", Label: "创建备份", Description: "导出指定数据库的表结构和数据，并保存为集群内备份 Secret。", Tone: "primary", Fields: []ToolWorkspaceActionField{{Name: "database", Label: "数据库名", Required: true, Placeholder: "postgres"}}},
			{Key: "create_table", Label: "创建表", Description: "在指定数据库中创建表。", Fields: sqlTableFields(true)},
			{Key: "drop_table", Label: "删除表", Description: "删除指定数据库中的表。", Tone: "danger", Fields: sqlTableFields(false)},
			{Key: "insert_table_row", Label: "新增行", Description: "向表插入一行 JSON 数据。", Fields: sqlRowFields("values", `{"column":"value"}`)},
			{Key: "update_table_row", Label: "更新行", Description: "根据 WHERE JSON 更新行。", Fields: append(sqlRowFields("values", `{"name":"updated"}`), ToolWorkspaceActionField{Name: "where", Label: "WHERE JSON", Type: "textarea", Required: true, Placeholder: `{"id":"1"}`})},
			{Key: "delete_table_row", Label: "删除行", Description: "根据 WHERE JSON 删除行。", Tone: "danger", Fields: []ToolWorkspaceActionField{
				{Name: "database", Label: "数据库名", Required: true, Placeholder: "appdb"},
				{Name: "table", Label: "表名", Required: true, Placeholder: "users"},
				{Name: "where", Label: "WHERE JSON", Type: "textarea", Required: true, Placeholder: `{"id":"1"}`},
			}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取运行资源。"},
		}
	}
	if inst.ServiceType == "redis" {
		actions = []ToolWorkspaceAction{
			{Key: "check_redis_health", Label: "检查 Redis", Description: "执行 Redis PING。", Tone: "primary"},
			{Key: "inspect_redis", Label: "实例信息", Description: "读取 Redis key 数量、版本和内存信息。"},
			{Key: "list_redis_keys", Label: "查看 Key", Description: "按模式扫描 Redis key。", Fields: []ToolWorkspaceActionField{
				{Name: "pattern", Label: "匹配模式", Placeholder: "*", Default: "*"},
				{Name: "limit", Label: "数量", Type: "number", Placeholder: "50", Default: "50"},
			}},
			{Key: "get_redis_key", Label: "读取 Key", Description: "读取 Redis key 的类型、TTL 和值。", Fields: []ToolWorkspaceActionField{{Name: "key", Label: "Key", Required: true, Placeholder: "session:1"}}},
			{Key: "set_redis_key", Label: "写入 Key", Description: "写入字符串 key，可选 TTL。", Fields: []ToolWorkspaceActionField{
				{Name: "key", Label: "Key", Required: true, Placeholder: "session:1"},
				{Name: "value", Label: "Value", Type: "textarea", Required: true, Placeholder: "value"},
				{Name: "ttlSeconds", Label: "TTL 秒", Type: "number", Placeholder: "3600"},
			}},
			{Key: "delete_redis_key", Label: "删除 Key", Description: "删除 Redis key。", Tone: "danger", Fields: []ToolWorkspaceActionField{{Name: "key", Label: "Key", Required: true, Placeholder: "session:1"}}},
			{Key: "expire_redis_key", Label: "设置 TTL", Description: "设置 Redis key 过期时间。", Fields: []ToolWorkspaceActionField{
				{Name: "key", Label: "Key", Required: true, Placeholder: "session:1"},
				{Name: "ttlSeconds", Label: "TTL 秒", Type: "number", Required: true, Placeholder: "3600"},
			}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取 Redis 信息。"},
		}
	}
	if inst.ServiceType == "minio" {
		actions = []ToolWorkspaceAction{
			{Key: "list_minio_buckets", Label: "查看桶", Description: "列出 MinIO buckets。", Tone: "primary"},
			{Key: "create_minio_bucket", Label: "创建桶", Description: "创建一个 bucket。", Fields: []ToolWorkspaceActionField{{Name: "bucket", Label: "Bucket", Required: true, Placeholder: "artifacts"}}},
			{Key: "delete_minio_bucket", Label: "删除桶", Description: "删除一个空 bucket。", Tone: "danger", Fields: []ToolWorkspaceActionField{{Name: "bucket", Label: "Bucket", Required: true, Placeholder: "artifacts"}}},
			{Key: "list_minio_objects", Label: "查看对象", Description: "列出 bucket 中的对象。", Fields: []ToolWorkspaceActionField{
				{Name: "bucket", Label: "Bucket", Required: true, Placeholder: "artifacts"},
				{Name: "prefix", Label: "Prefix", Placeholder: "releases/"},
				{Name: "limit", Label: "数量", Type: "number", Default: "50"},
			}},
			{Key: "delete_minio_object", Label: "删除对象", Description: "删除 bucket 中的对象。", Tone: "danger", Fields: []ToolWorkspaceActionField{
				{Name: "bucket", Label: "Bucket", Required: true, Placeholder: "artifacts"},
				{Name: "object", Label: "Object", Required: true, Placeholder: "releases/app.tar.gz"},
			}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取 MinIO 信息。"},
		}
	}
	if inst.ServiceType == "mongodb" {
		actions = []ToolWorkspaceAction{
			{Key: "list_mongodb_databases", Label: "查看数据库", Description: "列出 MongoDB 数据库。", Tone: "primary"},
			{Key: "list_mongodb_collections", Label: "查看集合", Description: "列出数据库集合。", Fields: []ToolWorkspaceActionField{{Name: "database", Label: "数据库", Required: true, Placeholder: "appdb"}}},
			{Key: "create_mongodb_collection", Label: "创建集合", Description: "创建 MongoDB collection。", Fields: []ToolWorkspaceActionField{
				{Name: "database", Label: "数据库", Required: true, Placeholder: "appdb"},
				{Name: "collection", Label: "集合", Required: true, Placeholder: "users"},
			}},
			{Key: "insert_mongodb_document", Label: "新增文档", Description: "插入 JSON 文档。", Fields: []ToolWorkspaceActionField{
				{Name: "database", Label: "数据库", Required: true, Placeholder: "appdb"},
				{Name: "collection", Label: "集合", Required: true, Placeholder: "users"},
				{Name: "document", Label: "文档 JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`},
			}},
			{Key: "update_mongodb_documents", Label: "更新文档", Description: "按 filter 更新文档。", Fields: []ToolWorkspaceActionField{
				{Name: "database", Label: "数据库", Required: true, Placeholder: "appdb"},
				{Name: "collection", Label: "集合", Required: true, Placeholder: "users"},
				{Name: "filter", Label: "Filter JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`},
				{Name: "update", Label: "Update JSON", Type: "textarea", Required: true, Placeholder: `{"status":"active"}`},
			}},
			{Key: "delete_mongodb_documents", Label: "删除文档", Description: "按 filter 删除文档。", Tone: "danger", Fields: []ToolWorkspaceActionField{
				{Name: "database", Label: "数据库", Required: true, Placeholder: "appdb"},
				{Name: "collection", Label: "集合", Required: true, Placeholder: "users"},
				{Name: "filter", Label: "Filter JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`},
			}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取 MongoDB 信息。"},
		}
	}
	if inst.ServiceType == "rabbitmq" {
		actions = []ToolWorkspaceAction{
			{Key: "list_rabbitmq_queues", Label: "查看队列", Description: "列出 RabbitMQ queues。", Tone: "primary"},
			{Key: "list_rabbitmq_exchanges", Label: "查看交换机", Description: "列出 RabbitMQ exchanges。"},
			{Key: "list_rabbitmq_vhosts", Label: "查看 VHost", Description: "列出 RabbitMQ virtual hosts。"},
			{Key: "list_rabbitmq_bindings", Label: "查看绑定", Description: "列出 RabbitMQ bindings。"},
			{Key: "create_rabbitmq_queue", Label: "创建队列", Description: "创建 RabbitMQ queue。", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Default: "/", Placeholder: "/"},
				{Name: "queue", Label: "队列", Required: true, Placeholder: "jobs"},
				{Name: "durable", Label: "持久化", Type: "checkbox", Default: "true"},
			}},
			{Key: "create_rabbitmq_exchange", Label: "创建交换机", Description: "创建 RabbitMQ exchange。", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Default: "/", Placeholder: "/"},
				{Name: "exchange", Label: "交换机", Required: true, Placeholder: "orders.events"},
				{Name: "type", Label: "类型", Default: "topic", Placeholder: "direct/topic/fanout/headers"},
				{Name: "durable", Label: "持久化", Type: "checkbox", Default: "true"},
			}},
			{Key: "create_rabbitmq_vhost", Label: "创建 VHost", Description: "创建 RabbitMQ virtual host。", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Required: true, Placeholder: "tenant-a"},
			}},
			{Key: "create_rabbitmq_binding", Label: "创建绑定", Description: "把交换机绑定到队列或另一个交换机。", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Default: "/", Placeholder: "/"},
				{Name: "source", Label: "源交换机", Required: true, Placeholder: "orders.events"},
				{Name: "destinationType", Label: "目标类型", Default: "queue", Placeholder: "queue 或 exchange"},
				{Name: "destination", Label: "目标", Required: true, Placeholder: "orders.created"},
				{Name: "routingKey", Label: "Routing key", Placeholder: "orders.#"},
				{Name: "arguments", Label: "Arguments JSON", Type: "textarea", Placeholder: `{"x-match":"all"}`},
			}},
			{Key: "publish_rabbitmq_message", Label: "发布消息", Description: "向指定 exchange 发布一条消息。", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Default: "/", Placeholder: "/"},
				{Name: "exchange", Label: "交换机", Placeholder: "orders.events"},
				{Name: "routingKey", Label: "Routing key", Placeholder: "orders.created"},
				{Name: "payload", Label: "消息内容", Type: "textarea", Required: true, Placeholder: `{"id":1}`},
				{Name: "properties", Label: "Properties JSON", Type: "textarea", Placeholder: `{"content_type":"application/json"}`},
			}},
			{Key: "delete_rabbitmq_queue", Label: "删除队列", Description: "删除 RabbitMQ queue。", Tone: "danger", Fields: []ToolWorkspaceActionField{
				{Name: "vhost", Label: "VHost", Default: "/", Placeholder: "/"},
				{Name: "queue", Label: "队列", Required: true, Placeholder: "jobs"},
			}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取 RabbitMQ 信息。"},
		}
	}
	if inst.ServiceType == "kafka" {
		actions = []ToolWorkspaceAction{
			{Key: "list_kafka_topics", Label: "查看 Topic", Description: "列出 Kafka topics。", Tone: "primary"},
			{Key: "create_kafka_topic", Label: "创建 Topic", Description: "创建 Kafka topic。", Fields: []ToolWorkspaceActionField{
				{Name: "topic", Label: "Topic", Required: true, Placeholder: "events"},
				{Name: "partitions", Label: "分区数", Type: "number", Default: "1"},
			}},
			{Key: "read_kafka_messages", Label: "读取消息", Description: "从指定 Topic 读取消息，不提交消费位点。", Fields: []ToolWorkspaceActionField{
				{Name: "topic", Label: "Topic", Required: true, Placeholder: "events"},
				{Name: "partition", Label: "Partition", Type: "number", Placeholder: "留空读取全部"},
				{Name: "offset", Label: "Offset", Default: "first", Placeholder: "first / latest / 0"},
				{Name: "limit", Label: "数量", Type: "number", Default: "10"},
			}},
			{Key: "produce_kafka_message", Label: "写入消息", Description: "向指定 Topic 写入一条消息。", Fields: []ToolWorkspaceActionField{
				{Name: "topic", Label: "Topic", Required: true, Placeholder: "events"},
				{Name: "key", Label: "Key", Placeholder: "order-1"},
				{Name: "value", Label: "消息内容", Type: "textarea", Required: true, Placeholder: `{"id":1}`},
				{Name: "partition", Label: "Partition", Type: "number", Placeholder: "留空自动分配"},
			}},
			{Key: "delete_kafka_topic", Label: "删除 Topic", Description: "删除 Kafka topic。", Tone: "danger", Fields: []ToolWorkspaceActionField{{Name: "topic", Label: "Topic", Required: true, Placeholder: "events"}}},
			{Key: "refresh", Label: "刷新资源", Description: "重新读取 Kafka 信息。"},
		}
	}
	return ToolWorkspace{
		Kind:        "data",
		Title:       "数据与中间件配置",
		Description: "查看连接信息、运行资源和组件接入配置。",
		Actions:     actions,
		Resources:   dataWorkspaceResources(inst),
		Config: append(baseWorkspaceConfig(inst),
			ToolWorkspaceConfig{Label: "接入方式", Value: "Service DNS + Secret 引用"},
		),
	}
}

func dataWorkspaceResources(inst model.ServiceInstallation) []ToolWorkspaceResource {
	name := valueOrDefault(inst.ReleaseName, inst.ServiceType)
	status := statusReadyOrPending(inst.Status)
	connection := ToolWorkspaceResource{
		Name:        name + "-connection",
		Type:        "Connection",
		Status:      status,
		Description: "Service DNS 和 Secret 凭据状态。",
		Annotations: map[string]interface{}{
			"namespace": inst.Namespace,
			"service":   accessURL(inst.Namespace, inst.ServiceType),
		},
	}
	return []ToolWorkspaceResource{connection}
}

func buildGenericWorkspace(inst model.ServiceInstallation) ToolWorkspace {
	return ToolWorkspace{
		Kind:        "generic",
		Title:       "工具配置",
		Description: "查看工具访问入口、安装参数和运行资源。",
		Actions:     []ToolWorkspaceAction{{Key: "refresh", Label: "刷新资源", Description: "重新读取运行资源。", Tone: "primary"}},
		Resources:   []ToolWorkspaceResource{{Name: valueOrDefault(inst.ReleaseName, inst.ServiceType), Type: "Release", Status: valueOrDefault(inst.Status, "Unknown"), Description: "当前工具安装实例。"}},
		Config:      baseWorkspaceConfig(inst),
	}
}

func componentIdentifierForWorkspace(comp model.Component) string {
	if comp.GitPath != "" {
		parts := strings.Split(strings.Trim(comp.GitPath, "/"), "/")
		return parts[len(parts)-1]
	}
	return ComponentIdentifier(comp.Name, comp.Type, comp.ID)
}

func accessURL(namespace, serviceType string) string {
	if namespace == "" {
		return "-"
	}
	serviceNames := map[string]string{
		"git":        namespace,
		"deploy":     namespace + "-argocd-server",
		"monitor":    namespace + "-grafana",
		"log":        namespace + "-loki",
		"ci":         namespace,
		"registry":   namespace,
		"harbor":     "harbor-portal",
		"mysql":      "mysql",
		"postgresql": "postgresql",
		"mongodb":    "mongodb",
		"redis":      "redis-master",
		"rabbitmq":   "rabbitmq",
		"kafka":      "kafka",
		"minio":      "minio",
	}
	name := serviceNames[serviceType]
	if name == "" {
		name = serviceType
	}
	port := map[string]string{
		"git": ":3000", "ci": ":8080", "registry": ":5000", "log": ":3100",
	}[serviceType]
	scheme := "http"
	if serviceType == "registry" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s.%s.svc.cluster.local%s", scheme, name, namespace, port)
}

func statusReadyOrPending(status string) string {
	if strings.EqualFold(status, "running") {
		return "Ready"
	}
	return "Pending"
}

func argoStatusFromComponent(comp model.Component) string {
	switch strings.ToLower(comp.Status) {
	case "running", "syncing":
		return "Synced"
	case "error", "failed":
		return "Degraded"
	default:
		return "Unknown"
	}
}

func argoHealthFromComponent(comp model.Component) string {
	switch strings.ToLower(comp.Status) {
	case "running", "syncing":
		return "Healthy"
	case "error", "failed":
		return "Degraded"
	default:
		return "Progressing"
	}
}

func componentBranch(comp model.Component) string {
	return valueOrDefault(comp.SourceBranch, "main")
}

func componentLanguage(comp model.Component) string {
	switch strings.ToLower(comp.Type) {
	case "frontend":
		return "Vue/Node"
	case "backend":
		return "Go"
	case "database":
		return "SQL"
	case "middleware":
		return "Kubernetes"
	default:
		return "Container/Kubernetes"
	}
}

func jenkinsStatusFromComponent(comp model.Component, inst model.ServiceInstallation) string {
	switch strings.ToLower(comp.PipelineStatus) {
	case "success", "succeeded", "ready":
		return "Success"
	case "failed", "failure", "error":
		return "Failed"
	case "running", "building":
		return "Running"
	case "unstable":
		return "Unstable"
	case "disabled":
		return "Disabled"
	default:
		return statusReadyOrPending(inst.Status)
	}
}

func jenkinsColorFromComponent(comp model.Component, inst model.ServiceInstallation) string {
	switch strings.ToLower(jenkinsStatusFromComponent(comp, inst)) {
	case "success", "ready":
		return "blue"
	case "failed":
		return "red"
	case "running":
		return "blue_anime"
	case "unstable":
		return "yellow"
	case "disabled":
		return "disabled"
	default:
		return "grey"
	}
}

func componentImageTag(comp model.Component) string {
	if comp.RegistryImage != "" {
		parts := strings.Split(comp.RegistryImage, ":")
		if len(parts) > 1 {
			return parts[len(parts)-1]
		}
	}
	return valueOrDefault(comp.Version, "latest")
}

func shouldExposeRegistryRepository(comp model.Component, tag string) bool {
	if strings.EqualFold(strings.TrimSpace(comp.Status), "draft") {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(tag), "latest") {
		return false
	}
	return strings.TrimSpace(comp.RegistryImage) != ""
}

func registryRepositoryURL(inst model.ServiceInstallation, repoName, serviceType string) string {
	if serviceType == "harbor" {
		parts := strings.SplitN(repoName, "/", 2)
		project := repoName
		repository := ""
		if len(parts) == 2 {
			project = parts[0]
			repository = parts[1]
		}
		return proxyURL(inst, fmt.Sprintf("/harbor/projects/%s/repositories/%s", project, repository))
	}
	return proxyURL(inst, "/v2/"+strings.Trim(repoName, "/")+"/tags/list")
}

func componentRegistryImageForWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, comp model.Component, repoName, tag string) string {
	image := strings.TrimSpace(comp.RegistryImage)
	if image == "" {
		return RuntimeRegistryImage(app, env, inst.ServiceType, repoName, tag)
	}
	currentHost := RuntimeRegistryHost(app, env, inst.ServiceType)
	if strings.HasPrefix(image, currentHost+"/") {
		return image
	}
	if strings.HasPrefix(image, "registry.paap.local:5000/") || strings.HasPrefix(image, "registry.paap.local/") {
		return RuntimeRegistryImage(app, env, inst.ServiceType, repoName, tag)
	}
	return image
}

func isEnvironmentRegistryImage(image, runtimeHost string) bool {
	image = strings.TrimSpace(image)
	runtimeHost = strings.TrimSpace(runtimeHost)
	if image == "" || runtimeHost == "" {
		return false
	}
	return strings.HasPrefix(image, runtimeHost+"/")
}

func proxyURL(inst model.ServiceInstallation, path string) string {
	if inst.EnvironmentID == 0 || inst.ID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/environments/%d/services/%d/proxy/%s", inst.EnvironmentID, inst.ID, strings.TrimLeft(path, "/"))
}

func runtimeRegistryTrustResource(inst model.ServiceInstallation, host string) ToolWorkspaceResource {
	endpoint := "https://" + strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(host), "https://"), "http://")
	host = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(host), "https://"), "http://")
	return ToolWorkspaceResource{
		Name:        host,
		Type:        "Runtime Trust",
		Status:      "Action Required",
		Description: "业务 Pod 拉取镜像前，需要管理员确保每个节点运行时能解析并信任该 HTTPS registry。",
		Annotations: map[string]interface{}{
			"registryHost":        host,
			"registryEndpoint":    endpoint,
			"containerdHostsToml": fmt.Sprintf("/etc/containerd/certs.d/%s/hosts.toml", host),
			"dockerCertPath":      fmt.Sprintf("/etc/docker/certs.d/%s/ca.crt", host),
			"certificateURL":      fmt.Sprintf("/api/v1/environments/%d/services/%d/registry-ca.crt", inst.EnvironmentID, inst.ID),
			"adminNote":           "PAAP 内部调用工具使用集群内 Service URL；只有业务镜像地址需要节点运行时可访问的 HTTPS registry host。",
			"agentManifest":       "deploy/k8s/paap-node-registry-agent.yaml",
		},
	}
}

func valueOrDash(value string) string {
	return valueOrDefault(value, "-")
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func argoCDApplicationFields() []ToolWorkspaceActionField {
	return []ToolWorkspaceActionField{
		{Name: "name", Label: "Application 名称", Required: true, Placeholder: "app-dev-api"},
		{Name: "repoURL", Label: "仓库地址", Required: true, Placeholder: "http://gitea/paap/app-dev-components.git"},
		{Name: "path", Label: "路径", Required: true, Placeholder: "components/api"},
		{Name: "targetRevision", Label: "Revision", Placeholder: "main", Default: "main"},
		{Name: "destinationNamespace", Label: "目标命名空间", Required: true, Placeholder: "app-dev"},
		{Name: "automated", Label: "自动同步", Type: "checkbox", Default: "true"},
	}
}

func argoCDApplicationSetFields() []ToolWorkspaceActionField {
	return []ToolWorkspaceActionField{
		{Name: "name", Label: "ApplicationSet 名称", Required: true, Placeholder: "app-dev-components"},
		{Name: "repoURL", Label: "仓库地址", Required: true, Placeholder: "http://gitea/paap/app-dev-components.git"},
		{Name: "path", Label: "目录匹配", Required: true, Placeholder: "components/*"},
		{Name: "targetRevision", Label: "Revision", Placeholder: "main", Default: "main"},
		{Name: "destinationNamespace", Label: "目标命名空间", Required: true, Placeholder: "app-dev"},
	}
}

func sqlTableFields(includeColumns bool) []ToolWorkspaceActionField {
	fields := []ToolWorkspaceActionField{
		{Name: "database", Label: "数据库名", Required: true, Placeholder: "appdb"},
		{Name: "table", Label: "表名", Required: true, Placeholder: "users"},
	}
	if includeColumns {
		fields = append(fields, ToolWorkspaceActionField{Name: "columns", Label: "字段定义", Type: "textarea", Required: true, Placeholder: "id:serial primary key\nname:varchar(120)\ncreated_at:timestamp"})
	}
	return fields
}

func sqlRowFields(valueName, valuePlaceholder string) []ToolWorkspaceActionField {
	return []ToolWorkspaceActionField{
		{Name: "database", Label: "数据库名", Required: true, Placeholder: "appdb"},
		{Name: "table", Label: "表名", Required: true, Placeholder: "users"},
		{Name: valueName, Label: "数据 JSON", Type: "textarea", Required: true, Placeholder: valuePlaceholder},
	}
}
