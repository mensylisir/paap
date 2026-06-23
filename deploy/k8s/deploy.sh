#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
KIND_CLUSTER="${KIND_CLUSTER:-kind}"
SERVER_IMAGE="${SERVER_IMAGE:-paap-server:v0.1.427}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-paap-operator:v0.1.52}"

echo "=== PAAP Deploy to Kind ==="

inspect_namespace() {
  namespace="$1"
  echo "   Pods in $namespace:"
  kubectl get pods -n "$namespace" -o wide || true
  echo "   Recent events in $namespace:"
  kubectl get events -n "$namespace" --sort-by=.lastTimestamp | tail -20 || true
}

# 1. 构建内置模板包和 PAAP 本地镜像
echo "1. Packaging templates and building PAAP images..."
cd "$PROJECT_DIR"
./scripts/package-built-in-templates.sh
docker build --build-arg FRONTEND_CACHE_BUST="$(date +%s)" -t "$SERVER_IMAGE" -f Dockerfile.server .
docker run --rm --entrypoint sh "$SERVER_IMAGE" -c 'test -x /paap-server && ls -l /paap-server'
docker build -t "$OPERATOR_IMAGE" -f Dockerfile.operator .

# 2. 在创建任何 Pod 之前，把所有镜像导入 kind。kind 集群不能访问外网时，这一步必须先完成。
echo "2. Preloading images into kind cluster '$KIND_CLUSTER'..."
"$PROJECT_DIR/scripts/preload-kind-images.sh"

# 3. 创建 namespace
echo "3. Creating namespaces..."
kubectl apply -f "$SCRIPT_DIR/namespace.yaml"

# 4. 安装 kpack。source 组件默认走 Buildpacks/kpack，必须先有 CRD/controller/webhook。
echo "4. Installing kpack v0.17.0..."
kubectl apply -f "$SCRIPT_DIR/kpack-v0.17.0.yaml"
inspect_namespace kpack

# 5. 安装 PAAP CRD
echo "5. Installing PAAP CRDs..."
kubectl apply -f "$PROJECT_DIR/config/crd/bases/"

# 6. 部署 PostgreSQL
echo "6. Deploying PostgreSQL..."
kubectl apply -f "$SCRIPT_DIR/postgres.yaml"

# 7. 部署 MinIO
echo "7. Deploying MinIO..."
kubectl apply -f "$SCRIPT_DIR/minio.yaml"

# 等待 MinIO 就绪
inspect_namespace paap-system

# 8. 初始化模板（data/charts/*.tar.gz 已进入 paap-server 镜像 /charts，再上传到 MinIO）
echo "8. Initializing templates..."
kubectl apply -f "$SCRIPT_DIR/init-templates.yaml"
echo "   Template initialization log:"
kubectl get job,pod -n paap-system -l job-name=init-templates -o wide || true
kubectl logs -n paap-system job/init-templates --tail=20 || true

# 9. 部署 Operator
echo "9. Deploying Operator..."
kubectl apply -f "$SCRIPT_DIR/paap-operator.yaml"

# 10. 部署 Server
echo "10. Deploying Server..."
kubectl apply -f "$SCRIPT_DIR/paap-server.yaml"

# 11. 直接检查当前状态，不做被动等待
echo "11. Inspecting current pod status..."
inspect_namespace paap-system
inspect_namespace kpack

echo ""
echo "=== PAAP Deployed Successfully ==="
echo ""
echo "Services:"
echo "  PAAP Server:    http://localhost:30091"
echo "  MinIO Console:  http://localhost:30901 (minioadmin/minioadmin123)"
echo "  PostgreSQL:     paap-postgres.paap-system.svc.cluster.local:5432"
echo ""
echo "CRDs installed:"
kubectl get crd | grep paap.io
echo ""
echo "Pods in paap-system:"
kubectl get pods -n paap-system
echo ""
echo "Pods in kpack:"
kubectl get pods -n kpack
echo ""
echo "Templates in MinIO:"
kubectl exec -n paap-system deployment/minio -- \
  mc alias set local http://localhost:9000 minioadmin minioadmin123 2>/dev/null || true
kubectl exec -n paap-system deployment/minio -- \
  mc ls local/paap-charts/charts/ 2>/dev/null || echo "  (Run 'kubectl port-forward -n paap-system svc/minio 9000:9000' to access MinIO)"
