# pbuf-registry Examples

This directory contains example protobuf files and a sample [`pbuf.yaml`](pbuf.yaml) configuration to help you get started with `pbuf-registry` and the `pbuf` CLI.

## What's Included

- **[`microservices-demo/`](microservices-demo/)** — A 1-click multi-service walkthrough (`billing-service` & `order-service`) demonstrating `pbuf modules push`, `pbuf vendor` with automatic `option go_package` patching (`gen_out`), and catching wire-breaking changes locally with `pbuf drift diff`.
- `hello-proto/` — A sample protobuf module with:
  - `greeter.proto` — A basic gRPC service definition
  - `types.proto` — Common message types and enums
- `pbuf.yaml` — Configuration file for the example module

## Prerequisites

1. **`pbuf-registry` running**: Follow the [Quick Start instructions](../README.md#quick-start-2-minutes) to start the registry locally.
2. **`pbuf` CLI installed**: Follow the [CLI installation instructions](../README.md#cli-installation-pbuf) or build from source (`make build-cli`).
3. **Authenticate with your registry token**:
   ```shell
   cd examples
   pbuf auth "${SERVER_STATIC_TOKEN}"
   ```

## Step-by-Step Tutorial

### Step 1: Register the Module

From the `examples/` directory, register the module defined in `pbuf.yaml` (`yourorg/hello-proto`):

```shell
cd examples
pbuf modules register
```

### Step 2: Push Your First Version

Push the `.proto` files from `hello-proto/` with semantic version tag `v1.0.0`:

```shell
pbuf modules push v1.0.0
```

This will:
- Read `pbuf.yaml`
- Collect all `.proto` files from the `export.paths` (`hello-proto/`) directory
- Upload them to the registry under immutable tag `v1.0.0`

### Step 3: Verify the Module

Retrieve module details, tags, and dependencies:

```shell
pbuf modules get yourorg/hello-proto
```

You can also inspect the parsed protobuf AST metadata (once the background proto-parser runs):

```shell
pbuf metadata get yourorg/hello-proto v1.0.0
```

### Step 4: Vendor Dependencies into a Consumer Project

Simulate consuming `yourorg/hello-proto` (alongside a `buf.build` or Git dependency) in another service:

```shell
mkdir -p /tmp/test-project
cd /tmp/test-project

cat > pbuf.yaml <<EOF
version: v1
name: yourorg/test-project
registry:
  addr: localhost:6777
  insecure: true
modules:
  - name: yourorg/hello-proto
    tag: v1.0.0
    out: third_party
EOF

pbuf vendor
```

The `.proto` files from `yourorg/hello-proto` (`v1.0.0`) will be downloaded into `/tmp/test-project/third_party/hello-proto/`.

### Step 5: Push a New Version & Check Drift

Return to `examples/`, modify `hello-proto/greeter.proto` (for example, add a new RPC method), and push `v1.1.0`:

```shell
cd /path/to/pbuf-registry/examples
pbuf modules push v1.1.0
```

Then check drift events:

```shell
pbuf drift module yourorg/hello-proto
```

## Troubleshooting

- **TLS / Self-Signed Certificate Errors**:
  - For local plaintext gRPC registries, set `insecure: true` under `registry:` in `pbuf.yaml`.
  - For TLS registries using `make cert-gen`, trust `gen/certs/ca-cert.pem` in your OS keychain/CA store.
- **Authentication Errors (`Unauthenticated` / `PermissionDenied`)**:
  - Re-run `pbuf auth <token>` in the directory containing your `pbuf.yaml` so `~/.netrc` has an entry matching `registry.addr`.
