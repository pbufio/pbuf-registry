#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

PBUF_BIN="${PROJECT_ROOT}/bin/pbuf"
REGISTRY_BIN="${PROJECT_ROOT}/bin/pbuf-registry"
MIGRATIONS_BIN="${PROJECT_ROOT}/bin/pbuf-migrations"

export DATA_DATABASE_DSN="${DATA_DATABASE_DSN:-postgres://pbuf:pbuf@localhost:5432/pbuf_registry?sslmode=disable}"
export SERVER_STATIC_TOKEN="${SERVER_STATIC_TOKEN:-e2e-admin-token}"
export SERVER_GRPC_TLS_ENABLED="false"
export SERVER_GRPC_AUTH_ENABLED="true"
export SERVER_GRPC_AUTH_TYPE="acl"
export SERVER_HTTP_AUTH_ENABLED="true"
export SERVER_HTTP_AUTH_TYPE="acl"

echo "==> Running database migrations..."
"${MIGRATIONS_BIN}"

echo "==> Starting pbuf-registry server..."
"${REGISTRY_BIN}" &
SERVER_PID=$!
cleanup() {
  if kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "==> Waiting for pbuf-registry healthz endpoint..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8082/healthz >/dev/null 2>&1; then
    echo "pbuf-registry is healthy!"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: pbuf-registry did not become healthy in time"
    exit 1
  fi
  sleep 1
done

WORK_DIR="$(mktemp -d)"
trap 'cleanup; rm -rf "${WORK_DIR}"' EXIT

echo "==> 1. Authenticating pbuf CLI against localhost:6777..."
cp -R "${PROJECT_ROOT}/api" "${WORK_DIR}/api"
cat > "${WORK_DIR}/pbuf.yaml" <<EOF
version: v1
name: pbufio/pbuf-registry
registry:
  addr: localhost:6777
  insecure: true
export:
  paths:
    - api
EOF

(
  cd "${WORK_DIR}"
  "${PBUF_BIN}" auth "${SERVER_STATIC_TOKEN}"
  echo "==> 2. Registering and pushing base module pbufio/pbuf-registry@v0.7.0..."
  "${PBUF_BIN}" modules register
  "${PBUF_BIN}" modules push v0.7.0
)

echo "==> 3. Testing examples/ workflow (register, push, get, list, vendor)..."
EXAMPLES_DIR="${WORK_DIR}/examples"
cp -R "${PROJECT_ROOT}/examples" "${EXAMPLES_DIR}"
rm -rf "${EXAMPLES_DIR}/api"

(
  cd "${EXAMPLES_DIR}"
  "${PBUF_BIN}" modules register
  "${PBUF_BIN}" modules push v1.0.0
  "${PBUF_BIN}" modules get yourorg/hello-proto
  "${PBUF_BIN}" modules list
  "${PBUF_BIN}" vendor
  test -f "${EXAMPLES_DIR}/api/pbuf-registry/v1/registry.proto"
)

echo "==> 4. Testing user/bot & ACL management via CLI..."
(
  cd "${EXAMPLES_DIR}"
  "${PBUF_BIN}" users create e2e-ci-bot --type bot
  "${PBUF_BIN}" users list
)

echo "==> 5. Verifying HTTP REST API..."
HTTP_MODULES="$(curl -sf -H "Authorization: Bearer ${SERVER_STATIC_TOKEN}" http://localhost:8080/v1/modules)"
echo "${HTTP_MODULES}" | grep -q "yourorg/hello-proto"
echo "${HTTP_MODULES}" | grep -q "pbufio/pbuf-registry"

echo "==> All E2E tests passed!"
