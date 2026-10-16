#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

PBUF_BIN="${PROJECT_ROOT}/bin/pbuf"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-pbuf-registry-e2e}"
export UI_PORT="${UI_PORT:-80}"
export SERVER_STATIC_TOKEN="${SERVER_STATIC_TOKEN:-dev-admin-token}"

WORK_DIR="$(mktemp -d)"
PBUF_HOME="${WORK_DIR}/home"
mkdir -p "${PBUF_HOME}"

cleanup() {
  local exit_code=$?
  if [ "${exit_code}" -ne 0 ]; then
    echo "==> Compose E2E failed; dumping container logs:"
    docker compose -f docker-compose.dev.yml logs || true
  fi
  echo "==> Stopping Docker Compose stack..."
  docker compose -f docker-compose.dev.yml down -v --remove-orphans || true
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

echo "==> Building and starting Docker Compose stack (docker-compose.dev.yml)..."
docker compose -f docker-compose.dev.yml up -d --build

echo "==> Waiting for pbuf-registry (http://localhost:8082/healthz)..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8082/healthz >/dev/null 2>&1; then
    echo "pbuf-registry is healthy!"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: pbuf-registry did not become healthy"
    exit 1
  fi
  sleep 1
done

echo "==> Waiting for pbuf-registry-ui (http://localhost:${UI_PORT}/health)..."
for i in $(seq 1 60); do
  if curl -sf "http://localhost:${UI_PORT}/health" >/dev/null 2>&1; then
    echo "pbuf-registry-ui is healthy!"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: pbuf-registry-ui did not become healthy"
    exit 1
  fi
  sleep 1
done

echo "==> Waiting for pbuf-registry-seed to finish seeding ACL tokens..."
for i in $(seq 1 60); do
  SEED_STATUS="$(docker compose -f docker-compose.dev.yml ps -a --format '{{.Service}}:{{.State}}:{{.ExitCode}}' | grep '^pbuf-registry-seed:' || true)"
  if echo "${SEED_STATUS}" | grep -q ':exited:0$'; then
    echo "pbuf-registry-seed completed!"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: pbuf-registry-seed did not finish cleanly (${SEED_STATUS})"
    exit 1
  fi
  sleep 1
done

echo "==> 1. Pushing pbufio/pbuf-registry@v0.7.0 with admin token via pbuf CLI..."
BASE_MOD_DIR="${WORK_DIR}/base-mod"
mkdir -p "${BASE_MOD_DIR}"
cp -R "${PROJECT_ROOT}/api" "${BASE_MOD_DIR}/api"
cat > "${BASE_MOD_DIR}/pbuf.yaml" <<EOF
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
  export HOME="${PBUF_HOME}"
  cd "${BASE_MOD_DIR}"
  "${PBUF_BIN}" auth "${SERVER_STATIC_TOKEN}"
  "${PBUF_BIN}" modules register
  "${PBUF_BIN}" modules push v0.7.0
)

echo "==> 2. Pushing yourorg/hello-proto@v1.0.0 with seeded writer token (pbuf_user_dev_writer_token)..."
EXAMPLES_DIR="${WORK_DIR}/examples"
cp -R "${PROJECT_ROOT}/examples" "${EXAMPLES_DIR}"
rm -rf "${EXAMPLES_DIR}/api"

(
  export HOME="${PBUF_HOME}"
  cd "${EXAMPLES_DIR}"
  "${PBUF_BIN}" auth "pbuf_user_dev_writer_token"
  "${PBUF_BIN}" modules register
  "${PBUF_BIN}" modules push v1.0.0
)

echo "==> 3. Testing seeded reader token (read-only access + ACL write rejection)..."
(
  export HOME="${PBUF_HOME}"
  cd "${EXAMPLES_DIR}"
  "${PBUF_BIN}" auth "pbuf_user_dev_reader_token"
  "${PBUF_BIN}" modules get yourorg/hello-proto
  "${PBUF_BIN}" modules list
  "${PBUF_BIN}" vendor
  test -f "${EXAMPLES_DIR}/api/pbuf-registry/v1/registry.proto"

  if "${PBUF_BIN}" modules push v2.0.0 2>/dev/null; then
    echo "ERROR: dev-reader should not be allowed to push modules"
    exit 1
  fi
)

echo "==> 4. Running Playwright UI E2E tests against live Docker Compose stack..."
(
  cd "${PROJECT_ROOT}/ui"
  test -d node_modules || npm ci
  PLAYWRIGHT_BASE_URL="http://127.0.0.1:${UI_PORT}" PLAYWRIGHT_LIVE_BACKEND=1 npm run test:e2e
)

echo "==> All Docker Compose + CLI + Playwright E2E tests passed!"
