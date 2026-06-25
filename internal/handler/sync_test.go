package handler

import (
	"context"
	"gorm.io/gorm"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sync/atomic"
	"testing"
	"time"

	"paap/internal/database"
	"paap/internal/k8s"
)

func TestSyncClusterStateIfPossibleSchedulesBackgroundSync(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	previousSync := syncClusterState
	previousLastSync := lastClusterSyncTime
	previousRunning := clusterSyncRunning
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
		syncClusterState = previousSync
		lastClusterSyncTime = previousLastSync
		clusterSyncRunning = previousRunning
	})

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	database.DB = db
	k8s.SetClient(fake.NewClientBuilder().Build())

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var calls int32
	syncClusterState = func(_ context.Context, _ *gorm.DB, _ client.Client) error {
		atomic.AddInt32(&calls, 1)
		close(started)
		<-release
		close(done)
		return nil
	}
	lastClusterSyncTime = time.Time{}
	clusterSyncRunning = false

	start := time.Now()
	syncClusterStateIfPossible()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("syncClusterStateIfPossible blocked for %s", elapsed)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background sync did not start")
	}

	syncClusterStateIfPossible()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("sync calls while running = %d, want 1", got)
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("background sync did not finish")
	}
}

func TestSyncClusterStateIfPossibleSkipsRecentCompletedSync(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	previousSync := syncClusterState
	previousLastSync := lastClusterSyncTime
	previousRunning := clusterSyncRunning
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
		syncClusterState = previousSync
		lastClusterSyncTime = previousLastSync
		clusterSyncRunning = previousRunning
	})

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	database.DB = db
	k8s.SetClient(fake.NewClientBuilder().Build())

	var calls int32
	syncClusterState = func(_ context.Context, _ *gorm.DB, _ client.Client) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	lastClusterSyncTime = time.Now().Add(-10 * time.Second)
	clusterSyncRunning = false

	syncClusterStateIfPossible()
	time.Sleep(20 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("sync calls after recent completed sync = %d, want 0", got)
	}
}
