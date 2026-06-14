package k8s

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	databaseBackupLabelKey       = "paap.io/resource-kind"
	databaseBackupLabelValue     = "database-backup"
	databaseBackupServiceTypeKey = "paap.io/service-type"
	databaseBackupDataKey        = "backup.json.gz"
	databaseBackupMetadataKey    = "metadata.json"
	maxDatabaseBackupSecretBytes = 900 * 1024
)

type DatabaseBackupMetadata struct {
	Name                string `json:"name"`
	Namespace           string `json:"namespace"`
	ServiceType         string `json:"serviceType"`
	Engine              string `json:"engine"`
	Database            string `json:"database"`
	SecretName          string `json:"secretName"`
	CreatedAt           string `json:"createdAt"`
	OriginalSizeBytes   int    `json:"originalSizeBytes"`
	CompressedSizeBytes int    `json:"compressedSizeBytes"`
	TableCount          int    `json:"tableCount"`
	RowCount            int    `json:"rowCount"`
	Storage             string `json:"storage"`
}

func StoreDatabaseBackup(ctx context.Context, namespace, serviceType string, document []byte, meta DatabaseBackupMetadata) (DatabaseBackupMetadata, error) {
	namespace = strings.TrimSpace(namespace)
	serviceType = strings.TrimSpace(serviceType)
	if namespace == "" || serviceType == "" {
		return meta, fmt.Errorf("namespace and service type are required")
	}
	cl, err := requireClient()
	if err != nil {
		return meta, err
	}
	compressed, err := gzipBytes(document)
	if err != nil {
		return meta, err
	}
	if len(compressed) > maxDatabaseBackupSecretBytes {
		return meta, fmt.Errorf("backup is %s after compression; platform backup storage is limited to %s, configure object storage before backing up this database", formatBackupBytes(len(compressed)), formatBackupBytes(maxDatabaseBackupSecretBytes))
	}
	now := time.Now().UTC()
	if strings.TrimSpace(meta.CreatedAt) == "" {
		meta.CreatedAt = now.Format(time.RFC3339)
	}
	meta.Namespace = namespace
	meta.ServiceType = serviceType
	meta.OriginalSizeBytes = len(document)
	meta.CompressedSizeBytes = len(compressed)
	meta.Storage = "Kubernetes Secret"
	name := databaseBackupSecretName(serviceType, meta.Database, now)
	meta.Name = name
	meta.SecretName = name
	metadataJSON, err := json.Marshal(meta)
	if err != nil {
		return meta, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"paap.io/managed-by":         "paap-server",
				databaseBackupLabelKey:       databaseBackupLabelValue,
				databaseBackupServiceTypeKey: serviceType,
			},
			Annotations: map[string]string{
				"paap.io/database":              meta.Database,
				"paap.io/engine":                meta.Engine,
				"paap.io/created-at":            meta.CreatedAt,
				"paap.io/original-size-bytes":   strconv.Itoa(meta.OriginalSizeBytes),
				"paap.io/compressed-size-bytes": strconv.Itoa(meta.CompressedSizeBytes),
				"paap.io/table-count":           strconv.Itoa(meta.TableCount),
				"paap.io/row-count":             strconv.Itoa(meta.RowCount),
				"paap.io/storage":               meta.Storage,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			databaseBackupDataKey:     compressed,
			databaseBackupMetadataKey: metadataJSON,
		},
	}
	if err := cl.Create(ctx, secret); err != nil {
		return meta, err
	}
	return meta, nil
}

func ListDatabaseBackups(ctx context.Context, namespace, serviceType string) ([]DatabaseBackupMetadata, error) {
	cl, err := requireClient()
	if err != nil {
		return nil, err
	}
	list := &corev1.SecretList{}
	if err := cl.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels{
		databaseBackupLabelKey:       databaseBackupLabelValue,
		databaseBackupServiceTypeKey: serviceType,
	}); err != nil {
		return nil, err
	}
	backups := make([]DatabaseBackupMetadata, 0, len(list.Items))
	for _, secret := range list.Items {
		meta := databaseBackupMetadataFromSecret(secret)
		if meta.SecretName == "" {
			meta.SecretName = secret.Name
		}
		if meta.Name == "" {
			meta.Name = secret.Name
		}
		backups = append(backups, meta)
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt > backups[j].CreatedAt
	})
	return backups, nil
}

func databaseBackupMetadataFromSecret(secret corev1.Secret) DatabaseBackupMetadata {
	var meta DatabaseBackupMetadata
	if raw := secret.Data[databaseBackupMetadataKey]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &meta)
	}
	annotations := secret.Annotations
	meta.Name = firstNonEmpty(meta.Name, secret.Name)
	meta.SecretName = firstNonEmpty(meta.SecretName, secret.Name)
	meta.Namespace = firstNonEmpty(meta.Namespace, secret.Namespace)
	meta.ServiceType = firstNonEmpty(meta.ServiceType, secret.Labels[databaseBackupServiceTypeKey])
	meta.Database = firstNonEmpty(meta.Database, annotations["paap.io/database"])
	meta.Engine = firstNonEmpty(meta.Engine, annotations["paap.io/engine"])
	meta.CreatedAt = firstNonEmpty(meta.CreatedAt, annotations["paap.io/created-at"], string(secret.CreationTimestamp.Format(time.RFC3339)))
	meta.Storage = firstNonEmpty(meta.Storage, annotations["paap.io/storage"], "Kubernetes Secret")
	meta.OriginalSizeBytes = firstNonZero(meta.OriginalSizeBytes, atoiAnnotation(annotations["paap.io/original-size-bytes"]))
	meta.CompressedSizeBytes = firstNonZero(meta.CompressedSizeBytes, atoiAnnotation(annotations["paap.io/compressed-size-bytes"]), len(secret.Data[databaseBackupDataKey]))
	meta.TableCount = firstNonZero(meta.TableCount, atoiAnnotation(annotations["paap.io/table-count"]))
	meta.RowCount = firstNonZero(meta.RowCount, atoiAnnotation(annotations["paap.io/row-count"]))
	return meta
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func databaseBackupSecretName(serviceType, database string, t time.Time) string {
	base := "paap-db-backup-" + dnsLabelPart(serviceType) + "-" + dnsLabelPart(database)
	if len(base) > 180 {
		base = base[:180]
	}
	return strings.Trim(base, "-") + "-" + t.Format("20060102-150405") + "-" + strconv.FormatInt(t.UnixNano()%1000000, 10)
}

func dnsLabelPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}

func atoiAnnotation(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func formatBackupBytes(value int) string {
	if value >= 1024*1024 {
		return fmt.Sprintf("%.1f MiB", float64(value)/(1024*1024))
	}
	if value >= 1024 {
		return fmt.Sprintf("%.1f KiB", float64(value)/1024)
	}
	return fmt.Sprintf("%d B", value)
}
