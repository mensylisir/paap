package main

import (
	"context"
	"log"
	"os"

	"paap/config"
	"paap/internal/database"
	"paap/internal/handler"
	"paap/internal/k8s"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// 初始化数据库
	if err := database.Init(cfg.DatabaseURL); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer database.Close()
	handler.SeedDefaultUsers()
	handler.SeedServiceCatalog()
	handler.SeedEnvTemplates()

	// 初始化 K8s client（可选，集群内运行时自动连接）
	if err := k8s.Init(); err != nil {
		log.Printf("K8s client init failed: %v (CR management disabled)", err)
	} else {
		log.Println("K8s client initialized successfully")
		if result, err := handler.SyncBuiltinTemplatesNow(context.Background(), true); err != nil {
			log.Printf("Built-in template sync failed: %v", err)
		} else {
			log.Printf("Built-in template sync completed: updated=%d", result.Updated)
		}
		if err := service.SyncClusterState(context.Background(), database.DB, k8s.GetClient()); err != nil {
			log.Printf("Cluster state sync failed: %v", err)
		}
	}

	r := gin.Default()
	handler.SetupRouter(r)

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("PAAP server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
		os.Exit(1)
	}
}
