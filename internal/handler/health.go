package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Version  string `json:"version"`
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "paap-backend",
		Version: "0.1.0",
	})
}
