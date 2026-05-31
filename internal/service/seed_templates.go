package service

import (
	"encoding/json"
	"log"

	paapv1 "paap/api/v1"
	"paap/internal/database"
	"paap/internal/model"
)

// SeedServiceTemplates populates the ServiceTemplate table with all tool and infra templates
func SeedServiceTemplates() {
	var count int64
	database.DB.Model(&model.ServiceTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	templates := []model.ServiceTemplate{
		// ===== 工具类 =====
		{
			Type:        "deploy",
			Name:        "部署服务",
			Category:    "tool",
			Description: "ArgoCD: GitOps 持续部署工具，管理应用的部署、版本、回滚",
			Icon:        "rocket",
			Installer:   "raw-yaml",
			RawYamlTemplate: argocdTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "version", Label: "ArgoCD 版本", Type: "select", Options: []string{"v2.9", "v2.10", "v2.11"}, Default: "v2.10"},
			}),
			DefaultValues: toJSON(map[string]string{"version": "v2.10"}),
			WorkloadRolePolicy: toJSON([]paapv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"services", "configmaps", "secrets", "persistentvolumeclaims"}, Verbs: []string{"*"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments", "statefulsets", "replicasets"}, Verbs: []string{"*"}},
				{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"*"}},
				{APIGroups: []string{"autoscaling"}, Resources: []string{"horizontalpodautoscalers"}, Verbs: []string{"*"}},
			}),
			InstallOrder: 10,
		},
		{
			Type:        "ci",
			Name:        "CI 服务",
			Category:    "tool",
			Description: "Tekton: 云原生 CI/CD 流水线，自动构建和测试代码",
			Icon:        "flow",
			Installer:   "raw-yaml",
			RawYamlTemplate: tektonTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "version", Label: "Tekton 版本", Type: "select", Options: []string{"v0.53", "v0.54", "v0.55"}, Default: "v0.55"},
			}),
			DefaultValues: toJSON(map[string]string{"version": "v0.55"}),
			WorkloadRolePolicy: toJSON([]paapv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "pods/log", "services", "configmaps", "secrets"}, Verbs: []string{"*"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets"}, Verbs: []string{"*"}},
				{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"*"}},
			}),
			InstallOrder: 20,
		},
		{
			Type:        "monitor",
			Name:        "监控服务",
			Category:    "tool",
			Description: "Prometheus + Grafana: 资源监控、指标采集、告警可视化",
			Icon:        "chart-line",
			Installer:   "raw-yaml",
			RawYamlTemplate: prometheusTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "retention", Label: "数据保留时间", Type: "select", Options: []string{"7d", "15d", "30d"}, Default: "15d"},
			}),
			DefaultValues: toJSON(map[string]string{"retention": "15d"}),
			WorkloadRolePolicy: toJSON([]paapv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "services", "endpoints", "configmaps"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"discovery.k8s.io"}, Resources: []string{"endpointslices"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get", "list", "watch"}},
			}),
			InstallOrder: 30,
		},
		{
			Type:        "log",
			Name:        "日志服务",
			Category:    "tool",
			Description: "Loki: 轻量级日志收集与查询，与 Grafana 集成",
			Icon:        "document",
			Installer:   "raw-yaml",
			RawYamlTemplate: lokiTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "retention", Label: "日志保留时间", Type: "select", Options: []string{"7d", "14d", "30d"}, Default: "14d"},
			}),
			DefaultValues: toJSON(map[string]string{"retention": "14d"}),
			WorkloadRolePolicy: toJSON([]paapv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "pods/log"}, Verbs: []string{"get", "list", "watch"}},
			}),
			InstallOrder: 40,
		},
		{
			Type:        "registry",
			Name:        "镜像仓库",
			Category:    "tool",
			Description: "Docker Registry: 轻量级容器镜像仓库，适合开发测试",
			Icon:        "cube",
			Installer:   "raw-yaml",
			RawYamlTemplate: registryTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "storage", Label: "存储大小", Type: "select", Options: []string{"5Gi", "10Gi", "20Gi"}, Default: "10Gi"},
			}),
			DefaultValues:      toJSON(map[string]string{"storage": "10Gi"}),
			WorkloadRolePolicy: "[]", // 不需要访问业务 namespace
			InstallOrder:       50,
		},
		{
			Type:        "harbor",
			Name:        "Harbor",
			Category:    "tool",
			Description: "Harbor: 企业级容器镜像仓库，支持镜像扫描和签名",
			Icon:        "cube",
			Installer:   "helm",
			ChartRepo:   "https://helm.goharbor.io",
			ChartName:   "harbor/harbor",
			ChartVersion: "1.14.0",
			DefaultValues: toJSON(map[string]string{
				"expose.type":             "nodePort",
				"expose.tls.enabled":      "false",
				"harborAdminPassword":     "Harbor12345",
				"persistence.enabled":     "false",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "harborAdminPassword", Label: "管理员密码", Type: "password", Default: "Harbor12345", Required: true},
				{Key: "persistence.enabled", Label: "启用持久存储", Type: "boolean", Default: "false"},
			}),
			WorkloadRolePolicy: "[]", // 不需要访问业务 namespace
			InstallOrder:       51,
		},
		{
			Type:        "jenkins",
			Name:        "Jenkins",
			Category:    "tool",
			Description: "Jenkins: 经典 CI/CD 服务器，支持丰富的插件生态",
			Icon:        "flow",
			Installer:   "helm",
			ChartRepo:   "https://charts.jenkins.io",
			ChartName:   "jenkins/jenkins",
			ChartVersion: "4.8.1",
			DefaultValues: toJSON(map[string]string{
				"controller.image.tag":     "2.492.4-jdk17",
				"controller.adminPassword":  "admin123",
				"controller.serviceType":    "NodePort",
				"persistence.enabled":       "false",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "controller.adminPassword", Label: "管理员密码", Type: "password", Default: "admin123", Required: true},
			}),
			WorkloadRolePolicy: toJSON([]paapv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "pods/log", "services", "configmaps", "secrets"}, Verbs: []string{"*"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets"}, Verbs: []string{"*"}},
				{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"*"}},
			}),
			InstallOrder: 25,
		},

		// ===== 基础设施 - 数据库 =====
		{
			Type:        "postgresql",
			Name:        "PostgreSQL",
			Category:    "infra",
			Description: "PostgreSQL: 功能强大的开源关系型数据库",
			Icon:        "database",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/postgresql",
			ChartVersion: "15.5.0",
			DefaultValues: toJSON(map[string]string{
				"auth.username":           "appuser",
				"auth.password":           "changeme123",
				"auth.database":           "appdb",
				"primary.persistence.size": "8Gi",
				"global.defaultStorageClass": "standard",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.username", Label: "用户名", Type: "string", Default: "appuser", Required: true},
				{Key: "auth.password", Label: "密码", Type: "password", Default: "changeme123", Required: true},
				{Key: "auth.database", Label: "数据库名", Type: "string", Default: "appdb"},
				{Key: "primary.persistence.size", Label: "存储大小", Type: "select", Options: []string{"1Gi", "5Gi", "10Gi", "20Gi", "50Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 100,
		},
		{
			Type:        "mysql",
			Name:        "MySQL",
			Category:    "infra",
			Description: "MySQL: 最流行的开源关系型数据库",
			Icon:        "database",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/mysql",
			ChartVersion: "9.12.0",
			DefaultValues: toJSON(map[string]string{
				"auth.username":           "appuser",
				"auth.password":           "changeme123",
				"auth.database":           "appdb",
				"primary.persistence.size": "8Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.username", Label: "用户名", Type: "string", Default: "appuser", Required: true},
				{Key: "auth.password", Label: "密码", Type: "password", Default: "changeme123", Required: true},
				{Key: "auth.database", Label: "数据库名", Type: "string", Default: "appdb"},
				{Key: "primary.persistence.size", Label: "存储大小", Type: "select", Options: []string{"1Gi", "5Gi", "10Gi", "20Gi", "50Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 101,
		},
		{
			Type:        "mongodb",
			Name:        "MongoDB",
			Category:    "infra",
			Description: "MongoDB: NoSQL 文档数据库",
			Icon:        "database",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/mongodb",
			ChartVersion: "14.0.0",
			DefaultValues: toJSON(map[string]string{
				"auth.username":           "appuser",
				"auth.password":           "changeme123",
				"auth.database":           "appdb",
				"persistence.size":        "8Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.username", Label: "用户名", Type: "string", Default: "appuser", Required: true},
				{Key: "auth.password", Label: "密码", Type: "password", Default: "changeme123", Required: true},
				{Key: "auth.database", Label: "数据库名", Type: "string", Default: "appdb"},
				{Key: "persistence.size", Label: "存储大小", Type: "select", Options: []string{"1Gi", "5Gi", "10Gi", "20Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 102,
		},
		{
			Type:        "kingbase",
			Name:        "人大金仓",
			Category:    "infra",
			Description: "人大金仓: 国产关系型数据库，兼容 PostgreSQL 协议",
			Icon:        "database",
			Installer:   "raw-yaml",
			RawYamlTemplate: kingbaseTemplate,
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "password", Label: "密码", Type: "password", Default: "changeme123", Required: true},
				{Key: "database", Label: "数据库名", Type: "string", Default: "appdb"},
				{Key: "storage", Label: "存储大小", Type: "select", Options: []string{"5Gi", "10Gi", "20Gi"}, Default: "10Gi"},
			}),
			DefaultValues: toJSON(map[string]string{"password": "changeme123", "database": "appdb", "storage": "10Gi"}),
			InstallOrder: 103,
		},

		// ===== 基础设施 - 缓存 =====
		{
			Type:        "redis",
			Name:        "Redis",
			Category:    "infra",
			Description: "Redis: 高性能内存缓存数据库",
			Icon:        "cloud",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/redis",
			ChartVersion: "18.6.0",
			DefaultValues: toJSON(map[string]string{
				"auth.enabled":    "false",
				"master.count":    "1",
				"master.persistence.size": "2Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.enabled", Label: "启用密码", Type: "boolean", Default: "false"},
				{Key: "auth.password", Label: "密码", Type: "password", Default: ""},
				{Key: "master.persistence.size", Label: "存储大小", Type: "select", Options: []string{"1Gi", "2Gi", "5Gi", "10Gi"}, Default: "2Gi"},
			}),
			InstallOrder: 110,
		},

		// ===== 基础设施 - 消息队列 =====
		{
			Type:        "rabbitmq",
			Name:        "RabbitMQ",
			Category:    "infra",
			Description: "RabbitMQ: 开源消息中间件，支持 AMQP 协议",
			Icon:        "network",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/rabbitmq",
			ChartVersion: "12.0.0",
			DefaultValues: toJSON(map[string]string{
				"auth.username":        "user",
				"auth.password":        "changeme123",
				"persistence.size":     "8Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.username", Label: "用户名", Type: "string", Default: "user", Required: true},
				{Key: "auth.password", Label: "密码", Type: "password", Default: "changeme123", Required: true},
				{Key: "persistence.size", Label: "存储大小", Type: "select", Options: []string{"2Gi", "5Gi", "8Gi", "20Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 120,
		},
		{
			Type:        "kafka",
			Name:        "Kafka",
			Category:    "infra",
			Description: "Apache Kafka: 分布式流处理平台",
			Icon:        "network",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/kafka",
			ChartVersion: "26.0.0",
			DefaultValues: toJSON(map[string]string{
				"kraft.enabled":              "true",
				"persistence.size":           "8Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "persistence.size", Label: "存储大小", Type: "select", Options: []string{"2Gi", "5Gi", "8Gi", "20Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 121,
		},

		// ===== 基础设施 - 对象存储 =====
		{
			Type:        "minio",
			Name:        "MinIO",
			Category:    "infra",
			Description: "MinIO: S3 兼容的对象存储服务",
			Icon:        "data-base",
			Installer:   "helm",
			ChartRepo:   "https://charts.bitnami.com/bitnami",
			ChartName:   "bitnami/minio",
			ChartVersion: "13.0.0",
			DefaultValues: toJSON(map[string]string{
				"auth.rootUser":        "minioadmin",
				"auth.rootPassword":    "minioadmin123",
				"persistence.size":     "8Gi",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "auth.rootUser", Label: "用户名", Type: "string", Default: "minioadmin", Required: true},
				{Key: "auth.rootPassword", Label: "密码", Type: "password", Default: "minioadmin123", Required: true},
				{Key: "persistence.size", Label: "存储大小", Type: "select", Options: []string{"5Gi", "10Gi", "20Gi", "50Gi"}, Default: "8Gi"},
			}),
			InstallOrder: 130,
		},

		// ===== 基础设施 - 注册中心 =====
		{
			Type:        "nacos",
			Name:        "Nacos",
			Category:    "infra",
			Description: "Nacos: 阿里巴巴开源的注册中心与配置中心",
			Icon:        "server",
			Installer:   "helm",
			ChartRepo:   "https://nacos-group.github.io/nacos-k8s",
			ChartName:   "nacos/nacos",
			ChartVersion: "0.1.0",
			DefaultValues: toJSON(map[string]string{
				"global.mode":          "standalone",
				"persistence.enabled":  "false",
			}),
			ConfigurableParams: toJSON([]ParamDef{
				{Key: "global.mode", Label: "运行模式", Type: "select", Options: []string{"standalone", "cluster"}, Default: "standalone"},
			}),
			InstallOrder: 140,
		},
	}

	for _, t := range templates {
		database.DB.Create(&t)
	}
	log.Printf("Seeded %d service templates", len(templates))
}

// SeedEnvTemplates populates the EnvTemplate table with predefined environment templates
func SeedEnvTemplates() {
	var count int64
	database.DB.Model(&model.EnvTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	templates := []model.EnvTemplate{
		{
			Name:        "开发环境标准",
			Description: "适用于日常开发，包含完整的 CI/CD 和监控能力",
			Services:    toJSON([]string{"deploy", "ci", "monitor"}),
			Infra:       toJSON([]string{"postgresql", "redis"}),
			ResourceCPU: "4核",
			ResourceMem: "8GB",
			ResourceDisk: "50GB",
		},
		{
			Name:        "测试环境标准",
			Description: "适用于集成测试，包含 CI/CD 和监控",
			Services:    toJSON([]string{"deploy", "ci", "monitor"}),
			Infra:       toJSON([]string{"postgresql", "redis"}),
			ResourceCPU: "8核",
			ResourceMem: "16GB",
			ResourceDisk: "100GB",
		},
		{
			Name:        "生产环境标准",
			Description: "适用于生产部署，只有 CD 和监控，无 CI",
			Services:    toJSON([]string{"deploy", "monitor"}),
			Infra:       toJSON([]string{"postgresql", "redis"}),
			ResourceCPU: "16核",
			ResourceMem: "32GB",
			ResourceDisk: "200GB",
		},
		{
			Name:        "轻量开发环境",
			Description: "最小化环境，只有部署服务",
			Services:    toJSON([]string{"deploy"}),
			Infra:       toJSON([]string{}),
			ResourceCPU: "2核",
			ResourceMem: "4GB",
			ResourceDisk: "20GB",
		},
		{
			Name:        "全栈开发环境",
			Description: "完整工具链 + 全部基础设施",
			Services:    toJSON([]string{"deploy", "ci", "monitor", "log", "registry"}),
			Infra:       toJSON([]string{"postgresql", "redis", "rabbitmq", "minio"}),
			ResourceCPU: "8核",
			ResourceMem: "16GB",
			ResourceDisk: "100GB",
		},
	}

	for _, t := range templates {
		database.DB.Create(&t)
	}
	log.Printf("Seeded %d env templates", len(templates))
}

type ParamDef struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Default  string   `json:"default,omitempty"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}

func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
