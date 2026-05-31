# 内置模板标准化改造方案

## 📋 当前状态

### ✅ 已完成
- `data/charts/` 下有 12 个打包好的模板（旧格式，包含 platform-manifest.yaml）
- `docs/examples/` 下有 2 个标准格式示例

### ❌ 待完成
1. **内置模板未按标准格式放到 `docs/examples/`**
2. **MinIO 未部署到 `paap-system`**
3. **缺少模板上传到 MinIO 的初始化脚本**
4. **代码中 `SeedServiceTemplates()` 使用旧格式**

---

## 🎯 目标架构

### 标准格式要求

所有内置模板都应该：
1. 放在 `docs/examples/built-in-templates/` 目录
2. 使用标准格式：`chart/ + platform-manifest.yaml + preset-values.yaml`
3. 部署时上传到 MinIO
4. 安装时从 MinIO 拉取

### 目录结构

```
paap/
├── docs/
│   └── examples/
│       ├── built-in-templates/          # 新建：内置模板
│       │   ├── argocd/
│       │   │   ├── chart/
│       │   │   ├── platform-manifest.yaml
│       │   │   ├── preset-values.yaml
│       │   │   └── README.md
│       │   ├── prometheus/
│       │   ├── redis/
│       │   ├── postgresql/
│       │   └── ...
│       ├── custom-prometheus-template/  # 已有：用户示例
│       └── bitnami-redis-template/      # 已有：用户示例
│
├── data/
│   └── charts/                          # 保留：已打包的模板
│       ├── argocd.tar.gz
│       ├── redis.tar.gz
│       └── ...
│
└── deploy/
    └── k8s/
        ├── minio.yaml                   # 新建：MinIO 部署
        └── init-templates.yaml          # 新建：模板初始化 Job
```

---

## 📝 实施步骤

### 步骤 1：解压现有模板到 docs/examples

```bash
#!/bin/bash
# scripts/extract-built-in-templates.sh

set -e

SOURCE_DIR="data/charts"
TARGET_DIR="docs/examples/built-in-templates"

mkdir -p "$TARGET_DIR"

for tarball in "$SOURCE_DIR"/*.tar.gz; do
    template_name=$(basename "$tarball" .tar.gz)
    echo "Extracting $template_name..."
    
    mkdir -p "$TARGET_DIR/$template_name"
    tar -xzf "$tarball" -C "$TARGET_DIR/$template_name" --strip-components=1
    
    # 创建 README.md
    cat > "$TARGET_DIR/$template_name/README.md" <<EOF
# $template_name 内置模板

这是 PAAP 平台的内置模板，使用标准格式。

## 结构

\`\`\`
$template_name/
├── chart/                      # Helm Chart
├── platform-manifest.yaml      # 平台元数据
├── preset-values.yaml          # 配置覆盖
└── README.md                   # 本文件
\`\`\`

## 使用

此模板在平台部署时会自动上传到 MinIO，用户可以直接在 UI 中安装。

## 参考

- [自定义模板开发指南](../../design/custom-template-guide.md)
- [模板系统总览](../../design/template-system-overview.md)
EOF
    
    echo "✓ Extracted $template_name"
done

echo "All templates extracted to $TARGET_DIR"
```

### 步骤 2：创建 MinIO 部署配置

```yaml
# deploy/k8s/minio.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: paap-storage
  labels:
    paap.io/component: storage

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: minio-pvc
  namespace: paap-storage
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

---
apiVersion: v1
kind: Secret
metadata:
  name: minio-secret
  namespace: paap-storage
type: Opaque
stringData:
  root-user: minioadmin
  root-password: minioadmin123

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio
  namespace: paap-storage
  labels:
    app: minio
spec:
  replicas: 1
  selector:
    matchLabels:
      app: minio
  template:
    metadata:
      labels:
        app: minio
    spec:
      containers:
      - name: minio
        image: minio/minio:RELEASE.2024-05-10T01-13-15Z
        args:
          - server
          - /data
          - --console-address
          - ":9001"
        env:
        - name: MINIO_ROOT_USER
          valueFrom:
            secretKeyRef:
              name: minio-secret
              key: root-user
        - name: MINIO_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: minio-secret
              key: root-password
        ports:
        - containerPort: 9000
          name: api
        - containerPort: 9001
          name: console
        volumeMounts:
        - name: data
          mountPath: /data
        livenessProbe:
          httpGet:
            path: /minio/health/live
            port: 9000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /minio/health/ready
            port: 9000
          initialDelaySeconds: 10
          periodSeconds: 5
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: minio-pvc

---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: paap-storage
  labels:
    app: minio
spec:
  type: ClusterIP
  ports:
  - port: 9000
    targetPort: 9000
    name: api
  - port: 9001
    targetPort: 9001
    name: console
  selector:
    app: minio

---
# 可选：NodePort 用于外部访问
apiVersion: v1
kind: Service
metadata:
  name: minio-external
  namespace: paap-storage
  labels:
    app: minio
spec:
  type: NodePort
  ports:
  - port: 9000
    targetPort: 9000
    nodePort: 30900
    name: api
  - port: 9001
    targetPort: 9001
    nodePort: 30901
    name: console
  selector:
    app: minio
```

### 步骤 3：创建模板初始化 Job

```yaml
# deploy/k8s/init-templates.yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: init-templates-script
  namespace: paap-system
data:
  init.sh: |
    #!/bin/bash
    set -e
    
    echo "Waiting for MinIO to be ready..."
    until curl -sf http://minio.paap-storage.svc.cluster.local:9000/minio/health/live; do
      echo "MinIO not ready, waiting..."
      sleep 5
    done
    echo "MinIO is ready!"
    
    # 配置 mc (MinIO Client)
    mc alias set paap-minio http://minio.paap-storage.svc.cluster.local:9000 minioadmin minioadmin123
    
    # 创建 bucket
    mc mb paap-minio/paap-charts --ignore-existing
    echo "Bucket 'paap-charts' created"
    
    # 上传所有模板
    echo "Uploading templates..."
    for template in /templates/*.tar.gz; do
      template_name=$(basename "$template")
      echo "Uploading $template_name..."
      mc cp "$template" "paap-minio/paap-charts/templates/$template_name"
      echo "✓ Uploaded $template_name"
    done
    
    echo "All templates uploaded successfully!"
    
    # 列出所有模板
    echo "Templates in MinIO:"
    mc ls paap-minio/paap-charts/templates/

---
apiVersion: batch/v1
kind: Job
metadata:
  name: init-templates
  namespace: paap-system
  labels:
    app: paap-init
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: paap-init
    spec:
      restartPolicy: OnFailure
      initContainers:
      # 等待 MinIO 就绪
      - name: wait-for-minio
        image: busybox:1.36
        command:
          - sh
          - -c
          - |
            until nc -z minio.paap-storage.svc.cluster.local 9000; do
              echo "Waiting for MinIO..."
              sleep 5
            done
            echo "MinIO is up!"
      containers:
      - name: upload-templates
        image: minio/mc:RELEASE.2024-05-09T17-04-24Z
        command: ["/bin/bash", "/scripts/init.sh"]
        volumeMounts:
        - name: script
          mountPath: /scripts
        - name: templates
          mountPath: /templates
      volumes:
      - name: script
        configMap:
          name: init-templates-script
          defaultMode: 0755
      - name: templates
        hostPath:
          path: /path/to/paap/data/charts  # 需要修改为实际路径
          type: Directory
```

### 步骤 4：更新部署脚本

```bash
# deploy/k8s/deploy.sh
#!/bin/bash

set -e

KIND_CLUSTER=${KIND_CLUSTER:-paap-dev}
NAMESPACE=paap-system

echo "Deploying PAAP to kind cluster: $KIND_CLUSTER"

# 1. 创建 namespace
echo "Creating namespace..."
kubectl --context kind-$KIND_CLUSTER apply -f namespace.yaml

# 2. 部署 PostgreSQL
echo "Deploying PostgreSQL..."
kubectl --context kind-$KIND_CLUSTER apply -f postgres.yaml

# 3. 部署 MinIO
echo "Deploying MinIO..."
kubectl --context kind-$KIND_CLUSTER apply -f minio.yaml

# 等待 MinIO 就绪
echo "Waiting for MinIO to be ready..."
kubectl --context kind-$KIND_CLUSTER wait --for=condition=ready pod -l app=minio -n paap-storage --timeout=300s

# 4. 初始化模板（上传到 MinIO）
echo "Initializing templates..."
# 需要先将 data/charts 目录挂载到 kind 节点
docker cp ../../data/charts $KIND_CLUSTER-control-plane:/tmp/paap-charts

# 更新 init-templates.yaml 中的路径
sed "s|/path/to/paap/data/charts|/tmp/paap-charts|g" init-templates.yaml | \
  kubectl --context kind-$KIND_CLUSTER apply -f -

# 等待初始化完成
kubectl --context kind-$KIND_CLUSTER wait --for=condition=complete job/init-templates -n $NAMESPACE --timeout=300s
echo "Templates initialized!"

# 5. 部署 Operator
echo "Deploying PAAP Operator..."
kubectl --context kind-$KIND_CLUSTER apply -f paap-operator.yaml

# 6. 部署 Server
echo "Deploying PAAP Server..."
kubectl --context kind-$KIND_CLUSTER apply -f paap-server.yaml

echo "Deployment complete!"
echo ""
echo "Access points:"
echo "  PAAP Server: http://localhost:30091"
echo "  MinIO Console: http://localhost:30901 (minioadmin/minioadmin123)"
```

### 步骤 5：更新 Server 代码

```go
// internal/service/seed_templates.go

// SeedServiceTemplates 从 MinIO 加载内置模板到数据库
func SeedServiceTemplates() {
	var count int64
	database.DB.Model(&model.ServiceTemplate{}).Count(&count)
	if count > 0 {
		log.Println("Templates already seeded, skipping")
		return
	}

	log.Println("Seeding service templates from MinIO...")

	// 内置模板列表
	builtInTemplates := []string{
		"argocd",
		"prometheus", // 注意：monitor.tar.gz 应该重命名为 prometheus.tar.gz
		"redis",
		"postgresql",
		"mysql",
		"mongodb",
		"rabbitmq",
		"kafka",
		"minio",
		"harbor",
		"jenkins",
		"loki",
	}

	for _, templateType := range builtInTemplates {
		if err := seedTemplateFromMinIO(templateType); err != nil {
			log.Printf("Failed to seed template %s: %v", templateType, err)
		} else {
			log.Printf("✓ Seeded template: %s", templateType)
		}
	}

	log.Println("Template seeding complete!")
}

func seedTemplateFromMinIO(templateType string) error {
	// S3 配置
	s3Client, err := k8s.NewS3Client(
		"minio.paap-storage.svc.cluster.local:9000",
		"minioadmin",
		"minioadmin123",
		"paap-charts",
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// 下载模板
	s3Key := fmt.Sprintf("templates/%s.tar.gz", templateType)
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("paap-template-%s-*.tar.gz", templateType))
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := s3Client.DownloadFile(s3Key, tmpFile.Name()); err != nil {
		return fmt.Errorf("failed to download from MinIO: %w", err)
	}

	// 解析 platform-manifest.yaml
	tmpFile.Seek(0, 0)
	manifestYaml, presetValues, _, _, err := extractAndValidateArchive(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	var manifest model.PlatformManifest
	if err := yaml.Unmarshal([]byte(manifestYaml), &manifest); err != nil {
		return fmt.Errorf("invalid platform-manifest.yaml: %w", err)
	}

	// 创建数据库记录
	manifestJSON, _ := json.Marshal(manifest)
	
	template := model.ServiceTemplate{
		Type:                 templateType,
		Name:                 manifest.Name,
		Category:             inferCategory(templateType),
		Description:          manifest.Description,
		Icon:                 inferIcon(templateType),
		IsCustom:             false, // 内置模板
		PlatformManifestJSON: string(manifestJSON),
		S3Bucket:             "paap-charts",
		S3Key:                s3Key,
		PresetValues:         presetValues,
		InstallOrder:         inferInstallOrder(templateType),
		Enabled:              true,
	}

	return database.DB.Create(&template).Error
}

func inferCategory(templateType string) string {
	tools := map[string]bool{
		"argocd": true, "prometheus": true, "jenkins": true,
		"harbor": true, "loki": true,
	}
	if tools[templateType] {
		return "tool"
	}
	return "infra"
}

func inferIcon(templateType string) string {
	icons := map[string]string{
		"argocd":     "rocket",
		"prometheus": "chart-line",
		"redis":      "database",
		"postgresql": "database",
		"mysql":      "database",
		"mongodb":    "database",
		"rabbitmq":   "message",
		"kafka":      "stream",
		"minio":      "cube",
		"harbor":     "cube",
		"jenkins":    "flow",
		"loki":       "document",
	}
	if icon, ok := icons[templateType]; ok {
		return icon
	}
	return "cube"
}

func inferInstallOrder(templateType string) int {
	order := map[string]int{
		"argocd":     10,
		"jenkins":    20,
		"prometheus": 30,
		"loki":       40,
		"harbor":     50,
		"postgresql": 100,
		"mysql":      100,
		"mongodb":    100,
		"redis":      100,
		"rabbitmq":   100,
		"kafka":      100,
		"minio":      100,
	}
	if o, ok := order[templateType]; ok {
		return o
	}
	return 100
}
```

---

## 📋 执行清单

### 立即执行

- [ ] 1. 运行 `scripts/extract-built-in-templates.sh` 解压模板到 `docs/examples/built-in-templates/`
- [ ] 2. 创建 `deploy/k8s/minio.yaml`
- [ ] 3. 创建 `deploy/k8s/init-templates.yaml`
- [ ] 4. 更新 `deploy/k8s/deploy.sh`
- [ ] 5. 更新 `internal/service/seed_templates.go`

### 验证

- [ ] 6. 本地测试部署
  ```bash
  cd deploy/k8s
  ./deploy.sh
  ```
- [ ] 7. 验证 MinIO 中的模板
  ```bash
  kubectl port-forward -n paap-storage svc/minio 9000:9000
  # 访问 http://localhost:9000
  # 登录：minioadmin / minioadmin123
  # 检查 paap-charts/templates/ 下是否有所有模板
  ```
- [ ] 8. 验证数据库中的模板
  ```bash
  kubectl exec -it -n paap-system deployment/paap-server -- \
    psql -h postgres -U paap -d paap -c "SELECT type, name, s3_key FROM service_templates;"
  ```

---

## 🎯 最终效果

### 部署流程

```
1. kubectl apply -f namespace.yaml
   ↓
2. kubectl apply -f postgres.yaml
   ↓
3. kubectl apply -f minio.yaml
   ↓ (等待 MinIO 就绪)
4. kubectl apply -f init-templates.yaml
   ↓ (Job 上传所有模板到 MinIO)
5. kubectl apply -f paap-operator.yaml
   ↓
6. kubectl apply -f paap-server.yaml
   ↓ (Server 启动时从 MinIO 加载模板到数据库)
7. 部署完成！
```

### 用户安装流程

```
1. 用户在 UI 选择模板（如 Redis）
   ↓
2. Server 从数据库读取模板信息
   ↓
3. Server 从 MinIO 下载 redis.tar.gz
   ↓
4. Server 解析 platform-manifest.yaml
   ↓
5. Server 创建 ServiceInstance CR
   ↓
6. Operator 安装 Redis
```

---

## 📝 注意事项

1. **模板命名**：`data/charts/monitor.tar.gz` 应该重命名为 `prometheus.tar.gz`
2. **路径挂载**：kind 集群需要挂载 `data/charts` 目录
3. **网络访问**：确保 Server 可以访问 `minio.paap-storage.svc.cluster.local:9000`
4. **凭证管理**：生产环境应该使用 Secret 管理 MinIO 凭证
5. **模板验证**：上传前应该验证所有模板的格式正确性

---

## 🔄 迁移计划

如果要将现有的 `raw-yaml` 和 `helm` 方式的模板迁移到标准格式：

1. 参考 [迁移路线图](migration-roadmap.md)
2. 逐步将每个模板转换为标准格式
3. 重新打包并上传到 MinIO
4. 更新数据库记录
