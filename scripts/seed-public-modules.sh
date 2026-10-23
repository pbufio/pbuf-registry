#!/usr/bin/env bash
# Seed a pbuf-registry instance (e.g. pbuf.cloud / pbuf.io or localhost:6777)
# with popular upstream Protobuf modules and multi-version dependency graphs.
#
# Usage:
#   PBUF_REGISTRY_ADDR=pbuf.cloud PBUF_TOKEN=<token> ./scripts/seed-public-modules.sh
#   PBUF_REGISTRY_ADDR=localhost:6777 PBUF_REGISTRY_INSECURE=true PBUF_TOKEN=<token> ./scripts/seed-public-modules.sh

set -euo pipefail

REGISTRY_ADDR="${PBUF_REGISTRY_ADDR:-pbuf.cloud}"
INSECURE="${PBUF_REGISTRY_INSECURE:-false}"

if [ -n "${PBUF_BIN:-}" ]; then
  PBUF_BIN="${PBUF_BIN}"
elif [ -x "./bin/pbuf" ]; then
  PBUF_BIN="$(pwd)/bin/pbuf"
elif command -v pbuf >/dev/null 2>&1; then
  PBUF_BIN="$(command -v pbuf)"
else
  echo "Building ./bin/pbuf..."
  make build-cli
  PBUF_BIN="$(pwd)/bin/pbuf"
fi

WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

if [ -n "${PBUF_TOKEN:-}" ]; then
  cat > "${WORKDIR}/pbuf.yaml" <<EOF
version: v1
name: seed/auth
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
EOF
  (cd "${WORKDIR}" && "${PBUF_BIN}" auth "${PBUF_TOKEN}")
fi

seed_from_git() {
  local module_name="$1"
  local repo_url="$2"
  local repo_path="$3"
  local git_ref="$4"
  local push_tag="$5"

  local mod_dir="${WORKDIR}/${module_name//\//_}_${push_tag}"
  mkdir -p "${mod_dir}"

  cat > "${mod_dir}/pbuf.yaml" <<EOF
version: v1
name: ${module_name}
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
export:
  paths:
    - proto
modules:
  - repository: ${repo_url}
    path: ${repo_path}
    tag: ${git_ref}
    out: proto/${repo_path}
EOF

  echo "==> Seeding ${module_name} (${push_tag}) from ${repo_url}@${git_ref} (${repo_path})..."
  (
    cd "${mod_dir}"
    "${PBUF_BIN}" vendor
    "${PBUF_BIN}" modules register || true
    "${PBUF_BIN}" modules push "${push_tag}" || true
  )
}

seed_from_bsr() {
  local module_name="$1"
  local bsr_module="$2"
  local bsr_path="$3"
  local bsr_ref="$4"
  local push_tag="$5"

  local mod_dir="${WORKDIR}/${module_name//\//_}_${push_tag}"
  mkdir -p "${mod_dir}"

  cat > "${mod_dir}/pbuf.yaml" <<EOF
version: v1
name: ${module_name}
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
export:
  paths:
    - proto
modules:
  - name: ${bsr_module}
    path: ${bsr_path}
    tag: ${bsr_ref}
    out: proto
EOF

  echo "==> Seeding ${module_name} (${push_tag}) from ${bsr_module}@${bsr_ref}..."
  (
    cd "${mod_dir}"
    "${PBUF_BIN}" vendor
    "${PBUF_BIN}" modules register || true
    "${PBUF_BIN}" modules push "${push_tag}" || true
  )
}

# 1. Seed pbufio/pbuf-registry itself
REPO_ROOT="$(pwd)"
echo "==> Seeding pbufio/pbuf-registry (v0.9.0, v0.10.0)..."
for tag in v0.9.0 v0.10.0; do
  mod_dir="${WORKDIR}/pbuf_registry_${tag}"
  mkdir -p "${mod_dir}"
  cp -R "${REPO_ROOT}/api" "${mod_dir}/api"
  cat > "${mod_dir}/pbuf.yaml" <<EOF
version: v1
name: pbufio/pbuf-registry
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
export:
  paths:
    - api
EOF
  (
    cd "${mod_dir}"
    "${PBUF_BIN}" modules register || true
    "${PBUF_BIN}" modules push "${tag}" || true
  )
done

# 2. Seed widely used upstream Protobuf modules
seed_from_bsr "googleapis/common-protos" "buf.build/googleapis/googleapis" "google/type" "main" "v1.3.2"
seed_from_bsr "googleapis/google-api-http" "buf.build/googleapis/googleapis" "google/api" "main" "v1.3.2"
seed_from_bsr "googleapis/google-rpc-status" "buf.build/googleapis/googleapis" "google/rpc" "main" "v1.3.2"

seed_from_bsr "bufbuild/protovalidate" "buf.build/bufbuild/protovalidate" "buf/validate" "v0.13.0" "v0.13.0"
seed_from_bsr "bufbuild/protovalidate" "buf.build/bufbuild/protovalidate" "buf/validate" "v0.14.0" "v0.14.0"

seed_from_bsr "grpc-ecosystem/grpc-gateway" "buf.build/grpc-ecosystem/grpc-gateway" "protoc-gen-openapiv2/options" "v2.20.0" "v2.20.0"
seed_from_bsr "grpc-ecosystem/grpc-gateway" "buf.build/grpc-ecosystem/grpc-gateway" "protoc-gen-openapiv2/options" "v2.22.0" "v2.22.0"

seed_from_bsr "envoyproxy/protoc-gen-validate" "buf.build/envoyproxy/protoc-gen-validate" "validate" "main" "v1.1.0"
seed_from_bsr "connectrpc/eliza" "buf.build/connectrpc/eliza" "connectrpc/eliza/v1" "main" "v1.0.0"

seed_from_git "open-telemetry/opentelemetry-proto" "https://github.com/open-telemetry/opentelemetry-proto" "opentelemetry/proto/common/v1" "v1.3.2" "v1.3.2"
seed_from_git "open-telemetry/opentelemetry-proto" "https://github.com/open-telemetry/opentelemetry-proto" "opentelemetry/proto/trace/v1" "v1.4.0" "v1.4.0"

seed_from_git "cloudevents/spec-proto" "https://github.com/cloudevents/spec" "cloudevents/formats/protobuf" "v1.0.2" "v1.0.2"

# 3. Seed realistic multi-service dependency & drift showcase modules
echo "==> Seeding acme/payment-service (v1.0.0 -> v1.1.0 -> v2.0.0 with drift)..."
PAY_DIR="${WORKDIR}/acme_payment"
mkdir -p "${PAY_DIR}/api/payment/v1"
cat > "${PAY_DIR}/pbuf.yaml" <<EOF
version: v1
name: acme/payment-service
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
export:
  paths:
    - api
EOF

cat > "${PAY_DIR}/api/payment/v1/payment.proto" <<'EOF'
syntax = "proto3";

package acme.payment.v1;

option go_package = "github.com/acme/payment-service/gen/payment/v1;paymentv1";

enum Currency {
  CURRENCY_UNSPECIFIED = 0;
  CURRENCY_USD = 1;
  CURRENCY_EUR = 2;
}

message Money {
  Currency currency = 1;
  int64 units = 2;
  int32 nanos = 3;
}

message ChargeRequest {
  string customer_id = 1;
  Money amount = 2;
  string idempotency_key = 3;
}

message ChargeResponse {
  string transaction_id = 1;
  string status = 2;
}

service PaymentService {
  rpc Charge(ChargeRequest) returns (ChargeResponse);
}
EOF

(
  cd "${PAY_DIR}"
  "${PBUF_BIN}" modules register || true
  "${PBUF_BIN}" modules push "v1.0.0" || true
)

# v1.1.0: additive non-breaking change
cat > "${PAY_DIR}/api/payment/v1/payment.proto" <<'EOF'
syntax = "proto3";

package acme.payment.v1;

option go_package = "github.com/acme/payment-service/gen/payment/v1;paymentv1";

enum Currency {
  CURRENCY_UNSPECIFIED = 0;
  CURRENCY_USD = 1;
  CURRENCY_EUR = 2;
  CURRENCY_GBP = 3;
}

message Money {
  Currency currency = 1;
  int64 units = 2;
  int32 nanos = 3;
}

message ChargeRequest {
  string customer_id = 1;
  Money amount = 2;
  string idempotency_key = 3;
  map<string, string> metadata = 4;
}

message ChargeResponse {
  string transaction_id = 1;
  string status = 2;
  string receipt_url = 3;
}

message RefundRequest {
  string transaction_id = 1;
  Money amount = 2;
}

message RefundResponse {
  string refund_id = 1;
}

service PaymentService {
  rpc Charge(ChargeRequest) returns (ChargeResponse);
  rpc Refund(RefundRequest) returns (RefundResponse);
}
EOF

(
  cd "${PAY_DIR}"
  "${PBUF_BIN}" modules push "v1.1.0" || true
)

# v2.0.0: intentional breaking change (type change + field removal) to showcase CRITICAL drift detection
cat > "${PAY_DIR}/api/payment/v1/payment.proto" <<'EOF'
syntax = "proto3";

package acme.payment.v1;

option go_package = "github.com/acme/payment-service/gen/payment/v1;paymentv1";

enum Currency {
  CURRENCY_UNSPECIFIED = 0;
  CURRENCY_USD = 1;
  CURRENCY_EUR = 2;
  CURRENCY_GBP = 3;
}

message Money {
  Currency currency = 1;
  string units = 2;
  int32 nanos = 3;
}

message ChargeRequest {
  string customer_id = 1;
  Money amount = 2;
  string idempotency_key = 3;
  map<string, string> metadata = 4;
}

message ChargeResponse {
  string transaction_id = 1;
  string receipt_url = 3;
}

service PaymentService {
  rpc Charge(ChargeRequest) returns (ChargeResponse);
}
EOF

(
  cd "${PAY_DIR}"
  "${PBUF_BIN}" modules push "v2.0.0" || true
)

# Seed acme/order-service depending on acme/payment-service@v1.0.0 and googleapis/common-protos@v1.3.2
echo "==> Seeding acme/order-service (v1.0.0)..."
ORDER_DIR="${WORKDIR}/acme_order"
mkdir -p "${ORDER_DIR}/api/order/v1"
cat > "${ORDER_DIR}/pbuf.yaml" <<EOF
version: v1
name: acme/order-service
registry:
  addr: ${REGISTRY_ADDR}
  insecure: ${INSECURE}
export:
  paths:
    - api
modules:
  - name: acme/payment-service
    tag: v1.0.0
    out: third_party
  - name: googleapis/common-protos
    tag: v1.3.2
    out: third_party
EOF

cat > "${ORDER_DIR}/api/order/v1/order.proto" <<'EOF'
syntax = "proto3";

package acme.order.v1;

option go_package = "github.com/acme/order-service/gen/order/v1;orderv1";

message LineItem {
  string sku = 1;
  int32 quantity = 2;
  int64 unit_price_cents = 3;
}

message CreateOrderRequest {
  string customer_id = 1;
  repeated LineItem items = 2;
  string payment_idempotency_key = 3;
}

message CreateOrderResponse {
  string order_id = 1;
  string payment_transaction_id = 2;
}

service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
}
EOF

(
  cd "${ORDER_DIR}"
  "${PBUF_BIN}" modules register || true
  "${PBUF_BIN}" modules push "v1.0.0" || true
)

echo "==> Done seeding modules on ${REGISTRY_ADDR}!"
