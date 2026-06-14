package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func HandleComponentConsole(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	componentID, _ := strconv.Atoi(c.Param("componentId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	cfg, _ := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier)
	if cfg == nil {
		cfg = &k8s.RuntimeConfig{Namespace: env.Namespace, WorkloadName: identifier}
	}
	target, err := k8s.ResolveRuntimeTarget(ctx, env.Namespace, cfg)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	streamRuntimeConsole(c, target)
}

func HandleServiceConsole(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	_, _, inst, _, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	cfg, _ := k8s.DiscoverNamespaceRuntimeConfig(ctx, inst.Namespace)
	target, err := k8s.ResolveRuntimeTarget(ctx, inst.Namespace, cfg)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	streamRuntimeConsole(c, target)
}

func streamRuntimeConsole(c *gin.Context, target k8s.RuntimeMetricsTarget) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("runtime console websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()

	writer := &lockedWebSocketWriter{conn: conn}
	_, _ = writer.Write([]byte(fmt.Sprintf("Opening console in %s/%s (%s)\r\n", target.Namespace, target.Pod, target.Container)))

	go func() {
		defer cancel()
		defer stdinWriter.Close()
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				_ = stdinWriter.CloseWithError(err)
				return
			}
			if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
				continue
			}
			if len(payload) == 0 {
				continue
			}
			if _, err := stdinWriter.Write(payload); err != nil {
				return
			}
		}
	}()

	if err := k8s.StreamPodConsole(ctx, target, stdinReader, writer, writer); err != nil {
		_, _ = writer.Write([]byte("\r\nConsole disconnected: " + err.Error() + "\r\n"))
	}
}

type lockedWebSocketWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *lockedWebSocketWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return 0, err
	}
	return len(data), nil
}
