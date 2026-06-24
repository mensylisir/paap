#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

KUBECONFIG="${KUBECONFIG:-}"
IMAGES_TAR="/tmp/paap-images-$(date +%s).tar"
SSH_USER="root"
SSH_PORT=22

usage() {
  echo "Usage: $0 --kubeconfig <path> [--ssh-user <user>] [--ssh-port <port>]"
  echo ""
  echo "Deploy PAAP to a remote air-gapped cluster."
  echo "  --kubeconfig   Path to kubeconfig for the target cluster (required)"
  echo "  --ssh-user     SSH user for node access (default: root)"
  echo "  --ssh-port     SSH port (default: 22)"
  echo ""
  echo "Example:"
  echo "  KUBECONFIG=~/.kube/cluster12.kubeconfig $0 --kubeconfig ~/.kube/cluster12.kubeconfig"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubeconfig) KUBECONFIG="$2"; shift 2 ;;
    --ssh-user)   SSH_USER="$2";   shift 2 ;;
    --ssh-port)   SSH_PORT="$2";   shift 2 ;;
    *)            echo "Unknown: $1"; usage ;;
  esac
done

if [ -z "$KUBECONFIG" ] || [ ! -f "$KUBECONFIG" ]; then
  echo "ERROR: --kubeconfig is required and must point to an existing file"
  usage
fi

KUBECTL="kubectl --kubeconfig $KUBECONFIG"

echo "=== PAAP Deploy to Remote Air-Gapped Cluster ==="
echo "Kubeconfig: $KUBECONFIG"
echo ""

# ── Step 1: Get all node IPs ──
echo "1. Discovering cluster nodes..."
NODE_IPS=$($KUBECTL get nodes -o jsonpath='{range .items[*]}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}')
if [ -z "$NODE_IPS" ]; then
  echo "ERROR: No nodes found in cluster"
  exit 1
fi
echo "   Nodes: $NODE_IPS"
echo ""

# ── Step 2: Save images to tar ──
# PAAP system images (minimum set for paap-system infrastructure).
# Canvas service images (kpack, buildpacks, ArgoCD, Harbor, etc.) can be
# loaded later when those services are installed via the PAAP UI.
IMAGES=(
  "postgres:15-alpine"
  "minio/minio:RELEASE.2025-09-07T16-13-09Z"
  "busybox:1.36"
  "minio/mc:RELEASE.2024-05-09T17-04-24Z"
  "quay.io/keycloak/keycloak:25.0.0"
  "paap-server:v0.1.500"
  "paap-operator:v0.1.52"
)
echo "2. Exporting ${#IMAGES[@]} PAAP system images to oci-archive..."
IMG_DIR="/tmp/paap-image-tars"
MAPPING_FILE="/tmp/paap-image-mapping.sh"
rm -rf "$IMG_DIR"
mkdir -p "$IMG_DIR"
rm -f "$MAPPING_FILE"

for IMG in "${IMAGES[@]}"; do
  SAFE_NAME=$(echo "$IMG" | tr '/:.' '---')
  ARCHIVE="$IMG_DIR/${SAFE_NAME}.tar"
  echo "   -> $IMG"
  skopeo copy "docker-daemon:${IMG}" "oci-archive:${ARCHIVE}:latest" 2>&1 | tail -1
  # Record mapping: safename -> original image reference
  echo "${SAFE_NAME}.tar:${IMG}" >> "$MAPPING_FILE"
done

# Combine all images and the mapping into a single tar
echo "   Creating combined archive..."
cp "$MAPPING_FILE" "$IMG_DIR/mapping.txt"
tar -cf "$IMAGES_TAR" -C "$IMG_DIR" .
echo "   Saved to $IMAGES_TAR ($(du -h "$IMAGES_TAR" | cut -f1))"
rm -rf "$IMG_DIR" "$MAPPING_FILE"
echo ""

# ── Step 3: Distribute to all nodes ──
echo "3. Distributing images to all nodes..."
for NODE_IP in $NODE_IPS; do
  echo "   -> $NODE_IP: SCP images..."
  scp -P "$SSH_PORT" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    "$IMAGES_TAR" "$SSH_USER@$NODE_IP:/tmp/paap-images.tar" 2>&1 | tail -1
  echo "      Importing into containerd..."
  ssh -p "$SSH_PORT" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    "$SSH_USER@$NODE_IP" \
    "cd /tmp && mkdir -p paap-load && cd paap-load && tar -xf /tmp/paap-images.tar && while IFS=: read -r FILE IMG_NAME; do echo \"   Importing \$IMG_NAME\"; ctr -n=k8s.io images import --base-name \"\$IMG_NAME\" \"\$FILE\" 2>&1 | tail -1; done < mapping.txt && cd / && rm -rf /tmp/paap-load /tmp/paap-images.tar && echo '   OK'" 2>&1
done
echo ""

# Clean up local tar
rm -f "$IMAGES_TAR"

# ── Step 4: Apply manifests ──
echo "4. Applying kpack CRDs..."
$KUBECTL apply -f "$SCRIPT_DIR/kpack-v0.17.0.yaml" 2>&1 | tail -5 || true

echo ""
echo "5. Applying PAAP CRDs..."
$KUBECTL apply -f "$PROJECT_DIR/config/crd/bases/" 2>&1 | tail -5

echo ""
echo "6. Creating namespace..."
$KUBECTL apply -f "$SCRIPT_DIR/namespace.yaml" 2>&1

echo ""
echo "7. Deploying PostgreSQL..."
$KUBECTL apply -f "$SCRIPT_DIR/postgres.yaml" 2>&1

echo ""
echo "8. Deploying MinIO..."
$KUBECTL apply -f "$SCRIPT_DIR/minio.yaml" 2>&1

echo ""
echo "9. Deploying Keycloak..."
$KUBECTL apply -f "$SCRIPT_DIR/keycloak.yaml" 2>&1

echo ""
echo "10. Waiting for infra pods..."
$KUBECTL wait --for=condition=ready pod -l app=paap-postgres -n paap-system --timeout=120s 2>&1 || true
$KUBECTL wait --for=condition=ready pod -l app=minio -n paap-system --timeout=120s 2>&1 || true

echo ""
echo "11. Initializing templates..."
$KUBECTL apply -f "$SCRIPT_DIR/init-templates.yaml" 2>&1

echo ""
echo "12. Deploying Operator..."
$KUBECTL apply -f "$SCRIPT_DIR/paap-operator.yaml" 2>&1

echo ""
echo "13. Deploying Server..."
$KUBECTL apply -f "$SCRIPT_DIR/paap-server.yaml" 2>&1

echo ""
echo "14. Waiting for all pods..."
$KUBECTL wait --for=condition=ready pod -l app=paap-operator -n paap-system --timeout=120s 2>&1 || true
$KUBECTL wait --for=condition=ready pod -l app=paap-server -n paap-system --timeout=120s 2>&1 || true

echo ""
echo "=== Deploy Summary ==="
echo ""
$KUBECTL get pods -n paap-system -o wide 2>&1
echo ""
echo "=== Done ==="
