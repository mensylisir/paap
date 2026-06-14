package handler

import (
	"log"

	"paap/internal/database"
	"paap/internal/model"
)

// SeedEnvTemplates populates the EnvTemplate table with predefined environment templates.
func SeedEnvTemplates() {
	templates := []model.EnvTemplate{
		{
			Name:         "开发环境标准",
			Description:  "适用于日常开发，包含代码仓库、镜像仓库、部署工具、监控和日志基座",
			Services:     toJSON(foundationServiceTypes()),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "4核",
			ResourceMem:  "8GB",
			ResourceDisk: "50GB",
		},
		{
			Name:         "测试环境标准",
			Description:  "适用于集成测试，包含代码仓库、镜像仓库、部署工具、监控和日志基座",
			Services:     toJSON(foundationServiceTypes()),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "8核",
			ResourceMem:  "16GB",
			ResourceDisk: "100GB",
		},
		{
			Name:         "生产环境标准",
			Description:  "适用于生产部署，包含代码仓库、镜像仓库、部署工具、监控和日志基座，不默认启用 CI",
			Services:     toJSON(foundationServiceTypes()),
			Infra:        toJSON([]string{"postgresql", "redis"}),
			ResourceCPU:  "16核",
			ResourceMem:  "32GB",
			ResourceDisk: "200GB",
		},
		{
			Name:         "轻量开发环境",
			Description:  "最小化环境，仍包含代码仓库、镜像仓库、部署工具、监控和日志基座",
			Services:     toJSON(foundationServiceTypes()),
			Infra:        toJSON([]string{}),
			ResourceCPU:  "2核",
			ResourceMem:  "4GB",
			ResourceDisk: "20GB",
		},
		{
			Name:         "全栈开发环境",
			Description:  "完整工具链 + 常用基础设施，CI 作为基座之外的可选增强",
			Services:     toJSON(appendServiceTypes(foundationServiceTypes(), "ci")),
			Infra:        toJSON([]string{"postgresql", "redis", "rabbitmq", "minio"}),
			ResourceCPU:  "8核",
			ResourceMem:  "16GB",
			ResourceDisk: "100GB",
		},
	}

	for _, t := range templates {
		var existing model.EnvTemplate
		if err := database.DB.Where("name = ?", t.Name).First(&existing).Error; err == nil {
			t.ID = existing.ID
			database.DB.Model(&existing).Updates(map[string]interface{}{
				"description":   t.Description,
				"services":      t.Services,
				"infra":         t.Infra,
				"resource_cpu":  t.ResourceCPU,
				"resource_mem":  t.ResourceMem,
				"resource_disk": t.ResourceDisk,
			})
			continue
		}
		database.DB.Create(&t)
	}
	log.Printf("Seeded or refreshed %d env templates", len(templates))
}
