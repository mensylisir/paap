#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
KIND_CLUSTER="${KIND_CLUSTER:-kind}"

echo "=== PAAP Deploy to Kind ==="

# 1. 创建 namespace
echo "1. Creating namespace..."
kubectl apply -f "$SCRIPT_DIR/namespace.yaml"

# 2. 安装 CRD
echo "2. Installing CRDs..."
kubectl apply -f "$PROJECT_DIR/config/crd/bases/"

# 3. 部署 PostgreSQL
echo "3. Deploying PostgreSQL..."
kubectl apply -f "$SCRIPT_DIR/postgres.yaml"

# 4. 构建镜像
echo "4. Building images..."
cd "$PROJECT_DIR"
docker build -t paap-server:latest -f Dockerfile.server .
docker build -t paap-operator:latest -f Dockerfile.operator .

# 5. 加载镜像到 kind
echo "5. Loading images into kind cluster '$KIND_CLUSTER'..."
kind load docker-image paap-server:latest --name "$KIND_CLUSTER"
kind load docker-image paap-operator:latest --name "$KIND_CLUSTER"

# 6. 部署 Operator
echo "6. Deploying Operator..."
kubectl apply -f "$SCRIPT_DIR/paap-operator.yaml"

# 7. 部署 Server
echo "7. Deploying Server..."
kubectl apply -f "$SCRIPT_DIR/paap-server.yaml"

# 8. 等待就绪
echo "8. Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod -l app=paap-postgres -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=paap-operator -n paap-system --timeout=60s
kubectl wait --for=condition=ready pod -l app=paap-server -n paap-system --timeout=60s

echo ""
echo "=== PAAP Deployed Successfully ==="
echo ""
echo "Services:"
echo "  PAAP Server:  http://localhost:30090"
echo "  PostgreSQL:   paap-postgres.paap-system.svc.cluster.local:5432"
echo ""
echo "CRDs installed:"
kubectl get crd | grep paap.io
echo ""
echo "Pods in paap-system:"
kubectl get pods -n paap-system
