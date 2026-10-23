#!/usr/bin/env bash
# 30-second interactive demo of pbuf-registry:
# 1. Pushes demo/billing-service@v1.0.0
# 2. Vendors demo/billing-service into demo/order-service with automatic go_package patching (gen_out)
# 3. Introduces a breaking wire-type change in billing.proto and catches it locally via `pbuf drift diff`

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
REGISTRY_ADDR="${PBUF_REGISTRY_ADDR:-localhost:6777}"
INSECURE="${PBUF_REGISTRY_INSECURE:-true}"

if [ -x "${REPO_ROOT}/bin/pbuf" ]; then
  PBUF_BIN="${REPO_ROOT}/bin/pbuf"
elif command -v pbuf >/dev/null 2>&1; then
  PBUF_BIN="$(command -v pbuf)"
else
  echo "==> Building pbuf CLI..."
  (cd "${REPO_ROOT}" && make build-cli)
  PBUF_BIN="${REPO_ROOT}/bin/pbuf"
fi

export PBUF_REGISTRY_ADDR="${REGISTRY_ADDR}"
export PBUF_REGISTRY_INSECURE="${INSECURE}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
cp -R "${SCRIPT_DIR}/billing-service" "${WORKDIR}/billing-service"
cp -R "${SCRIPT_DIR}/order-service" "${WORKDIR}/order-service"

echo "=================================================================="
echo "1. Registering and pushing demo/billing-service@v1.0.0"
echo "=================================================================="
(
  cd "${WORKDIR}/billing-service"
  "${PBUF_BIN}" modules register || true
  "${PBUF_BIN}" modules push v1.0.0 || true
)

echo ""
echo "=================================================================="
echo "2. Vendoring demo/billing-service into demo/order-service (with gen_out go_package patching)"
echo "=================================================================="
(
  cd "${WORKDIR}/order-service"
  "${PBUF_BIN}" vendor
  echo ""
  echo "--- Patched go_package in third_party/api/billing/v1/billing.proto ---"
  grep "option go_package" third_party/api/billing/v1/billing.proto
)

echo ""
echo "=================================================================="
echo "3. Simulating a breaking wire change in billing.proto & running 'pbuf drift diff'"
echo "=================================================================="
cat > "${WORKDIR}/billing-service/api/billing/v1/billing.proto" <<'EOF'
syntax = "proto3";

package demo.billing.v1;

option go_package = "github.com/acme/billing-service/internal/pb/billing/v1;billingv1";

message Invoice {
  string invoice_id = 1;
  string customer_id = 2;
  // BREAKING: changed field #3 from int64 to string and removed field #4 (currency)
  string total_cents = 3;
}

message CreateInvoiceRequest {
  string customer_id = 1;
  string total_cents = 2;
}

message CreateInvoiceResponse {
  Invoice invoice = 1;
}

service BillingService {
  rpc CreateInvoice(CreateInvoiceRequest) returns (CreateInvoiceResponse);
}
EOF

(
  cd "${WORKDIR}/billing-service"
  "${PBUF_BIN}" drift diff --against v1.0.0 --format markdown --fail-on none
)
