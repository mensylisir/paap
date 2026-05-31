#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
KIND_CLUSTER="${KIND_CLUSTER:-kind}"

echo "=== PAAP Deploy to Kind ==="

# 1. 创建 namespace
echo "1. Creating namespaces..."
kubectl apply -f "$SCRIPT_DIR/namespace.yaml"

# 2. 安装 CRD
echo "2. Installing CRDs..."
kubectl apply -f "$PROJECT_DIR/config/crd/bases/"

# 3. 部署 PostgreSQL
echo "3. Deploying PostgreSQL..."
kubectl apply -f "$SCRIPT_DIR/postgres.yaml"

# 4. 部署 MinIO
echo "4. Deploying MinIO..."
kubectl apply -f "$SCRIPT_DIR/minio.yaml"

# 等待 MinIO 就绪
echo "   Waiting for MinIO to be ready..."
kubectl wait --for=condition=ready pod -l app=minio -n paap-system --timeout=300s
echo "   ✓ MinIO is ready"

# 5. 初始化模板（上传到 MinIO）
echo "5. Initializing templates..."

# 将模板目录复制到 kind 节点
echo "   Copying templates to kind node..."
docker cp "$PROJECT_DIR/data/charts" "$KIND_CLUSTER-control-plane:/tmp/paap-charts"

# 应用初始化 Job
kubectl apply -f "$SCRIPT_DIR/init-templates.yaml"

# 等待初始化完成
echo "   Waiting for template initialization..."
kubectl wait --for=condition=complete job/init-templates -n paap-system --timeout=300s
echo "   ✓ Templates initialized"

# 显示初始化日志
echo "   Template initialization log:"
kubectl logs -n paap-system job/init-templates --tail=20

# 6. 构建镜像
echo "6. Building images..."
cd "$PROJECT_DIR"
docker build -t paap-server:latest -f Dockerfile.server .
docker build -t paap-operator:latest -f Dockerfile.operator .

# 7. 加载镜像到 kind
echo "7. Loading images into kind cluster '$KIND_CLUSTER'..."
kind load docker-image paap-server:latest --name "$KIND_CLUSTER"
kind load docker-image paap-operator:latest --name "$KIND_CLUSTER"

# 8. 部署 Operator
echo "8. Deploying Operator..."
kubectl apply -f "$SCRIPT_DIR/paap-operator.yaml"

# 9. 部署 Server
echo "9. Deploying Server..."
kubectl apply -f "$SCRIPT_DIR/paap-server.yaml"

# 10. 等待就绪
echo "10. Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod -l app=paap-postgres -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=paap-operator -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=paap-server -n paap-system --timeout=60s

echo ""
echo "=== PAAP Deployed Successfully ==="
echo ""
echo "Services:"
echo "  PAAP Server:    http://localhost:30090"
echo "  MinIO Console:  http://localhost:30901 (minioadmin/minioadmin123)"
echo "  PostgreSQL:     paap-postgres.paap-system.svc.cluster.local:5432"
echo ""
echo "CRDs installed:"
kubectl get crd | grep paap.io
echo ""
echo "Pods in paap-system:"
kubectl get pods -n paap-system
echo ""
echo "Templates in MinIO:"
kubectl exec -n paap-system deployment/minio -- \
  mc alias set local http://localhost:9000 minioadmin minioadmin123 2>/dev/null || true
kubectl exec -n paap-system deployment/minio -- \
  mc ls local/paap-charts/templates/ 2>/dev/null || echo "  (Run 'kubectl port-forward -n paap-system svc/minio 9000:9000' to access MinIO)"
