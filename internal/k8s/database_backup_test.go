package k8s

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStoreAndListDatabaseBackupsUseRealSecrets(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })
	SetClient(fake.NewClientBuilder().WithScheme(scheme).Build())

	ctx := context.Background()
	created, err := StoreDatabaseBackup(ctx, "orders-dev-postgresql", "postgresql", []byte(`{"tables":[]}`), DatabaseBackupMetadata{
		Engine:     "postgresql",
		Database:   "postgres",
		CreatedAt:  "2026-06-14T00:00:00Z",
		TableCount: 2,
		RowCount:   5,
	})
	if err != nil {
		t.Fatalf("store backup: %v", err)
	}
	if created.SecretName == "" || created.CompressedSizeBytes == 0 || created.OriginalSizeBytes == 0 {
		t.Fatalf("backup metadata not populated: %#v", created)
	}

	backups, err := ListDatabaseBackups(ctx, "orders-dev-postgresql", "postgresql")
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(backups))
	}
	if backups[0].SecretName != created.SecretName || backups[0].Database != "postgres" || backups[0].TableCount != 2 || backups[0].RowCount != 5 {
		t.Fatalf("listed backup mismatch: %#v", backups[0])
	}
}
