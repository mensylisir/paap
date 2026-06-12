package handler

import (
	"context"
	"log"
	"sync"
	"time"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/service"
)

var (
	clusterSyncMu       sync.Mutex
	lastClusterSyncTime time.Time
	clusterSyncRunning  bool
	syncClusterState    = service.SyncClusterState
)

const clusterSyncMinInterval = 30 * time.Second

func syncClusterStateIfPossible() {
	cl := k8s.GetClient()
	db := database.DB
	if cl == nil || db == nil {
		return
	}

	clusterSyncMu.Lock()
	if clusterSyncRunning || time.Since(lastClusterSyncTime) < clusterSyncMinInterval {
		clusterSyncMu.Unlock()
		return
	}
	lastClusterSyncTime = time.Now()
	clusterSyncRunning = true
	clusterSyncMu.Unlock()

	go func() {
		defer func() {
			clusterSyncMu.Lock()
			clusterSyncRunning = false
			clusterSyncMu.Unlock()
		}()
		if err := syncClusterState(context.Background(), db, cl); err != nil {
			log.Printf("[syncClusterStateIfPossible] cluster sync failed: %v", err)
		}
	}()
}

func syncClusterStateNow() {
	cl := k8s.GetClient()
	db := database.DB
	if cl == nil || db == nil {
		return
	}

	clusterSyncMu.Lock()
	if clusterSyncRunning {
		clusterSyncMu.Unlock()
		return
	}
	clusterSyncRunning = true
	clusterSyncMu.Unlock()

	defer func() {
		clusterSyncMu.Lock()
		lastClusterSyncTime = time.Now()
		clusterSyncRunning = false
		clusterSyncMu.Unlock()
	}()
	if err := syncClusterState(context.Background(), db, cl); err != nil {
		log.Printf("[syncClusterStateNow] cluster sync failed: %v", err)
	}
}
