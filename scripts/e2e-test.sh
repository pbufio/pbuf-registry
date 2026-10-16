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
export HOME="${WORK_DIR}/home"
mkdir -p "${HOME}"
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

echo "==> 6. Running proto-parsing --once & verifying metadata via CLI and HTTP..."
"${REGISTRY_BIN}" proto-parsing --once
(
  cd "${EXAMPLES_DIR}"
  META_OUT="$("${PBUF_BIN}" metadata get yourorg/hello-proto v1.0.0)"
  echo "${META_OUT}" | grep -q "Greeter"
  echo "${META_OUT}" | grep -q "SayHello"
)
HTTP_META="$(curl -sf -X POST -H "Authorization: Bearer ${SERVER_STATIC_TOKEN}" -H "Content-Type: application/json" -d '{"name":"yourorg/hello-proto","tag":"v1.0.0"}' http://localhost:8080/v1/metadata)"
echo "${HTTP_META}" | grep -q "Greeter"

echo "==> 7. Triggering breaking change & testing drift list / module / dependencies / acknowledge..."
(
  cd "${WORK_DIR}"
  rm -f "${WORK_DIR}/api/pbuf-registry/v1/drift.proto"
  "${PBUF_BIN}" modules push v0.8.0
)
"${REGISTRY_BIN}" proto-parsing --once
(
  cd "${EXAMPLES_DIR}"
  DRIFT_LIST="$("${PBUF_BIN}" drift list)"
  echo "${DRIFT_LIST}" | grep -q "api/pbuf-registry/v1/drift.proto"

  DRIFT_MOD="$("${PBUF_BIN}" drift module pbufio/pbuf-registry --tag v0.8.0)"
  echo "${DRIFT_MOD}" | grep -q "api/pbuf-registry/v1/drift.proto"

  DRIFT_DEPS="$("${PBUF_BIN}" drift dependencies yourorg/hello-proto --tag v1.0.0)"
  echo "${DRIFT_DEPS}" | grep -q "pbufio/pbuf-registry"

  EVENT_ID="$(printf '%s' "${DRIFT_LIST}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["events"][0]["id"])')"
  ACK_OUT="$("${PBUF_BIN}" drift acknowledge "${EVENT_ID}" --by e2e-ci)"
  echo "${ACK_OUT}" | grep -q '"acknowledged": true'
  echo "${ACK_OUT}" | grep -q '"acknowledged_by": "e2e-ci"'

  # Default --unacknowledged-only=true should exclude the acknowledged event
  if "${PBUF_BIN}" drift list | grep -q "${EVENT_ID}"; then
    echo "ERROR: acknowledged drift event ${EVENT_ID} still returned by default drift list"
    exit 1
  fi
  # --unacknowledged-only=false should include the acknowledged event
  "${PBUF_BIN}" drift list --unacknowledged-only=false | grep -q "${EVENT_ID}"
)

echo "==> 8. Testing draft tag push and compaction --once..."
(
  cd "${EXAMPLES_DIR}"
  "${PBUF_BIN}" modules push v1.1.0-draft --draft
)
"${REGISTRY_BIN}" compaction --once

echo "==> 9. Testing pbuf vendor with go.mod and gen_out go_package patching..."
cat > "${EXAMPLES_DIR}/go.mod" <<EOF
module github.com/yourorg/hello-proto

go 1.25
EOF
cat > "${EXAMPLES_DIR}/pbuf.yaml" <<EOF
version: v1
name: yourorg/hello-proto
registry:
  addr: localhost:6777
  insecure: true
export:
  paths:
    - hello-proto
modules:
  - name: pbufio/pbuf-registry
    tag: v0.7.0
    gen_out: gen
EOF
(
  cd "${EXAMPLES_DIR}"
  rm -rf "${EXAMPLES_DIR}/api"
  "${PBUF_BIN}" vendor
  grep -q 'option go_package = "github.com/yourorg/hello-proto/gen/api/pbuf-registry/v1;v1";' "${EXAMPLES_DIR}/api/pbuf-registry/v1/registry.proto"
)

echo "==> All E2E tests passed!"
