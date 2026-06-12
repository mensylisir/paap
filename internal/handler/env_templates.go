package handler

import (
	"log"

	"paap/internal/database"
	"paap/internal/model"
)

// SeedEnvTemplates populates the EnvTemplate table with predefined environment templates.
func SeedEnvTemplates() {
	var count int64
	database.DB.Model(&model.EnvTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	templates := []model.EnvTemplate{
		{
			Name:         "开发环境标准",
			Description:  "适用于日常开发，包含完整的部署、代码仓库、镜像仓库和监控能力",
			Services:     toJSON([]string{"deploy", "git", "registry", "monitor", "log"}),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "4核",
			ResourceMem:  "8GB",
			ResourceDisk: "50GB",
		},
		{
			Name:         "测试环境标准",
			Description:  "适用于集成测试，包含部署、代码仓库、镜像仓库、监控和日志",
			Services:     toJSON([]string{"deploy", "git", "registry", "monitor", "log"}),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "8核",
			ResourceMem:  "16GB",
			ResourceDisk: "100GB",
		},
		{
			Name:         "生产环境标准",
			Description:  "适用于生产部署，包含部署、监控和日志，不默认启用 CI",
			Services:     toJSON([]string{"deploy", "monitor", "log"}),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "16核",
			ResourceMem:  "32GB",
			ResourceDisk: "200GB",
		},
		{
			Name:         "轻量开发环境",
			Description:  "最小化环境，包含部署服务、轻量代码仓库和轻量镜像仓库",
			Services:     toJSON([]string{"deploy", "git", "registry"}),
			Infra:        toJSON([]string{}),
			ResourceCPU:  "2核",
			ResourceMem:  "4GB",
			ResourceDisk: "20GB",
		},
		{
			Name:         "全栈开发环境",
			Description:  "完整工具链 + 常用基础设施",
			Services:     toJSON([]string{"deploy", "git", "registry", "ci", "monitor", "log"}),
			Infra:        toJSON([]string{"postgresql", "redis", "rabbitmq", "minio"}),
			ResourceCPU:  "8核",
			ResourceMem:  "16GB",
			ResourceDisk: "100GB",
		},
	}

	for _, t := range templates {
		database.DB.Create(&t)
	}
	log.Printf("Seeded %d env templates", len(templates))
}
