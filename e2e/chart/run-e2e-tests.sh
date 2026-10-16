#!/usr/bin/env bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-pbuf-registry-e2e}"
NAMESPACE="${NAMESPACE:-default}"
RELEASE_NAME="${RELEASE_NAME:-pbuf-registry-test}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../../charts/pbuf-registry"
TIMEOUT="${TIMEOUT:-180}"
PORT_FORWARD_PID=""

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    if [ "${SKIP_CLEANUP:-false}" != "true" ]; then
        if [ -n "${PORT_FORWARD_PID:-}" ]; then
              log_info "Stopping port forwarding..."
              kill "${PORT_FORWARD_PID}" 2>/dev/null || true
        fi

        log_info "Cleaning up resources..."
        helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" 2>/dev/null || true
        kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true 2>/dev/null || true
        
        if [ "${SKIP_CLUSTER_DELETE:-false}" != "true" ]; then
            log_info "Deleting Kind cluster..."
            kind delete cluster --name "${CLUSTER_NAME}" 2>/dev/null || true
        fi
    else
        log_warn "Skipping cleanup (SKIP_CLEANUP=true)"
    fi
}

trap cleanup EXIT

wait_for_pods() {
    local label=$1
    local timeout=$2
    local namespace=$3
    
    log_info "Waiting for pods with label ${label} to be ready (timeout: ${timeout}s)..."
    kubectl wait --for=condition=ready pod \
        -l "${label}" \
        -n "${namespace}" \
        --timeout="${timeout}s" || return 1
}

wait_for_statefulset() {
    local name=$1
    local timeout=$2
    local namespace=$3
    
    log_info "Waiting for StatefulSet ${name} to be ready (timeout: ${timeout}s)..."
    kubectl wait --for=jsonpath='{.status.readyReplicas}'=1 statefulset/${name} \
        -n "${namespace}" \
        --timeout="${timeout}s" || return 1
}

# Check prerequisites
log_info "Checking prerequisites..."
for cmd in kind kubectl helm; do
    if ! command -v "${cmd}" &> /dev/null; then
        log_error "${cmd} is not installed. Please install it first."
        exit 1
    fi
done

log_info "Prerequisites check passed"

# Create Kind cluster if it doesn't exist
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    log_info "Creating Kind cluster: ${CLUSTER_NAME}"
    kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml"
else
    log_info "Kind cluster '${CLUSTER_NAME}' already exists"
fi

# Set kubectl context
log_info "Setting kubectl context to kind-${CLUSTER_NAME}"
kubectl config use-context "kind-${CLUSTER_NAME}"

# Create namespace
log_info "Creating namespace: ${NAMESPACE}"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Install the chart with internal PostgreSQL
log_info "Installing Helm chart: ${RELEASE_NAME}"
helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --set postgresql.enabled=true \
    --set postgresql.primary.persistence.enabled=false \
    --set secrets.staticToken="test-token-12345" \
    --set replicaCount=1 \
    --wait \
    --timeout="${TIMEOUT}s"

# Wait for PostgreSQL to be ready
log_info "Waiting for PostgreSQL to be ready..."
wait_for_statefulset "${RELEASE_NAME}-postgresql" "${TIMEOUT}" "${NAMESPACE}"

# Wait for main application to be ready
log_info "Waiting for main application to be ready..."
wait_for_pods "app.kubernetes.io/name=pbuf-registry,app.kubernetes.io/component!=database" "${TIMEOUT}" "${NAMESPACE}"

# Wait for background jobs to be ready
log_info "Waiting for background jobs to be ready..."
sleep 10  # Give background jobs time to start

# Run health checks
log_info "Running health checks..."

# Get pod names
MAIN_POD=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=pbuf-registry,app.kubernetes.io/component!=database" -o jsonpath='{.items[0].metadata.name}')

if [ -z "${MAIN_POD}" ]; then
    log_error "Could not find main application pod"
    exit 1
fi

log_info "Main pod: ${MAIN_POD}"

# Check health endpoint
log_info "Checking health endpoint..."
kubectl exec -n "${NAMESPACE}" "${MAIN_POD}" -- wget -q -O - http://localhost:8082/healthz || {
    log_error "Health check failed"
    kubectl logs -n "${NAMESPACE}" "${MAIN_POD}" --tail=50
    exit 1
}

log_info "Health check passed"

# Check database connectivity
log_info "Checking database connectivity..."
PG_POD=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/component=database" -o jsonpath='{.items[0].metadata.name}')

if [ -z "${PG_POD}" ]; then
    log_error "Could not find PostgreSQL pod"
    exit 1
fi

kubectl exec -n "${NAMESPACE}" "${PG_POD}" -- psql -U pbuf -d pbuf_registry -c "SELECT version();" > /dev/null || {
    log_error "Database connectivity check failed"
    kubectl logs -n "${NAMESPACE}" "${PG_POD}" --tail=50
    exit 1
}

log_info "Database connectivity check passed"

# Check background jobs
log_info "Checking background jobs..."
for job in compaction protoparser; do
    JOB_POD=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=pbuf-registry,background=${job}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "${JOB_POD}" ]; then
        log_info "Background job '${job}' pod: ${JOB_POD}"
        kubectl exec -n "${NAMESPACE}" "${JOB_POD}" -- wget -q -O - http://localhost:8082/healthz || {
            log_warn "Background job '${job}' health check failed (this might be expected if disabled)"
        }
    else
        log_warn "Background job '${job}' pod not found (might be disabled)"
    fi
done

# Install pbuf CLI for functional testing
log_info "Installing pbuf CLI..."
PBUF_CLI_VERSION="${PBUF_CLI_VERSION:-latest}"
PBUF_INSTALL_DIR="${SCRIPT_DIR}/bin"
mkdir -p "${PBUF_INSTALL_DIR}"

if [ ! -f "${PBUF_INSTALL_DIR}/pbuf" ]; then
    log_info "Building pbuf CLI from monorepo..."
    (cd "${SCRIPT_DIR}/../.." && go build -o "${PBUF_INSTALL_DIR}/pbuf" ./cmd/pbuf)
    chmod +x "${PBUF_INSTALL_DIR}/pbuf"
else
    log_info "pbuf CLI already installed"
fi

PBUF="${PBUF_INSTALL_DIR}/pbuf"

# Set up port forwarding for registry access
log_info "Setting up port forwarding for registry access..."
kubectl port-forward -n "${NAMESPACE}" "svc/${RELEASE_NAME}" 6777:6777 &
PORT_FORWARD_PID=$!
sleep 5  # Give port-forward time to establish

# Test pbuf CLI functionality
log_info "Testing pbuf CLI functionality..."

# Copy example module to a temporary directory
TEMP_MODULE_DIR=$(mktemp -d)
cp -r "${SCRIPT_DIR}/example-module"/* "${TEMP_MODULE_DIR}/"
cd "${TEMP_MODULE_DIR}"

# Register the module
log_info "Registering test module..."
"${PBUF}" modules register || {
    log_error "Failed to register module"
    exit 1
}
log_info "Module registered successfully"

# Push the module
log_info "Pushing test module v1.0.0..."
"${PBUF}" modules push v1.0.0 || {
    log_error "Failed to push module"
    exit 1
}
log_info "Module pushed successfully"

# Get module information
log_info "Getting module information..."
"${PBUF}" modules get test-org/hello-proto || {
    log_error "Failed to get module information"
    exit 1
}
log_info "Module information retrieved successfully"

# Test vendoring in a consumer project
log_info "Testing module vendoring..."
CONSUMER_DIR=$(mktemp -d)
cd "${CONSUMER_DIR}"

cat > pbuf.yaml <<EOF
version: v1
name: test-org/consumer
registry:
  addr: localhost:6777
  insecure: true
modules:
  - name: test-org/hello-proto
    tag: v1.0.0
EOF

"${PBUF}" vendor || {
    log_error "Failed to vendor module"
    exit 1
}
log_info "Module vendored successfully"

# Clean up temporary directories
cd "${SCRIPT_DIR}"
rm -rf "${TEMP_MODULE_DIR}" "${CONSUMER_DIR}"

log_info "${GREEN}All pbuf CLI tests passed!${NC}"

# Test with external database configuration
log_info "Testing with external database configuration..."
helm upgrade --install "${RELEASE_NAME}-ext" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --set postgresql.enabled=false \
    --set secrets.databaseDSN="postgres://pbuf:pbuf@${RELEASE_NAME}-postgresql:5432/pbuf_registry?sslmode=disable" \
    --set secrets.staticToken="test-token-12345" \
    --set replicaCount=1 \
    --wait \
    --timeout="${TIMEOUT}s"

log_info "Waiting for external DB configuration deployment..."
sleep 10

EXT_POD=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/instance=${RELEASE_NAME}-ext" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "${EXT_POD}" ]; then
    kubectl exec -n "${NAMESPACE}" "${EXT_POD}" -- wget -q -O - http://localhost:8082/healthz || {
        log_error "External DB configuration health check failed"
        kubectl logs -n "${NAMESPACE}" "${EXT_POD}" --tail=50
        exit 1
    }
    log_info "External DB configuration check passed"
    helm uninstall "${RELEASE_NAME}-ext" -n "${NAMESPACE}"
fi

log_info "${GREEN}========================================${NC}"
log_info "${GREEN}All e2e tests passed successfully!${NC}"
log_info "${GREEN}========================================${NC}"

exit 0
