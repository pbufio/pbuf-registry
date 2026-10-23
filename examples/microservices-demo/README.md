# 30-Second Microservices Demo (`billing-service` & `order-service`)

This self-contained example demonstrates the three core workflows of `pbuf-registry` across two Go gRPC microservices (`demo/billing-service` and `demo/order-service`):

1. **Publishing a Producer Schema (`pbuf modules push`)**: `billing-service` registers and publishes `v1.0.0` of `api/billing/v1/billing.proto`.
2. **Consumer Vendoring with Automatic `go_package` Patching (`gen_out`)**:
   - `billing-service` declares `option go_package = "github.com/acme/billing-service/internal/pb/billing/v1;billingv1";`.
   - `order-service` has its own `go.mod` (`module github.com/acme/order-service`) and sets `gen_out: gen` in `pbuf.yaml`.
   - Running `pbuf vendor` inside `order-service` pulls `billing.proto` into `third_party/` and automatically rewrites `option go_package` to `"github.com/acme/order-service/gen/api/billing/v1"`.
3. **Catching Wire-Breaking Changes Locally (`pbuf drift diff`)**:
   - `billing-service` modifies field `#3` (`total_cents`) from `int64` to `string` and deletes field `#4` (`currency`).
   - Running `pbuf drift diff --against v1.0.0 --format markdown` immediately flags `FIELD_TYPE_CHANGED` and `FIELD_REMOVED` as `CRITICAL` before the breaking tag is pushed.

## Run Locally

Start the local registry stack (or point `PBUF_REGISTRY_ADDR` at an existing instance):

```bash
# From the repository root:
make run

# Run the 30-second microservices walkthrough:
./examples/microservices-demo/run-demo.sh
```
