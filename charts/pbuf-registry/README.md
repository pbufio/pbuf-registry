# pbuf-registry Helm Chart

![Version: 0.21.0](https://img.shields.io/badge/Version-0.21.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.19.0](https://img.shields.io/badge/AppVersion-v0.19.0-informational?style=flat-square)

Official Kubernetes Helm chart for deploying [`pbuf-registry`](https://github.com/pbufio/pbuf-registry) — including the Registry Server, background workers (compaction and proto-parsing/drift detection), optional Web UI (`pbuf-registry-ui`), and optional internal PostgreSQL.

---

## Installation

### Option 1: From GHCR OCI Registry (Recommended)

```shell
helm install my-pbuf-registry oci://ghcr.io/pbufio/charts/pbuf-registry \
  --version 0.21.0 \
  --set secrets.staticToken="your-admin-token"
```

### Option 2: From Helm Repository

```shell
helm repo add pbuf https://pbufio.github.io/pbuf-registry
helm repo update

helm install my-pbuf-registry pbuf/pbuf-registry \
  --set secrets.staticToken="your-admin-token"
```

### Option 3: From Local Monorepo Checkout

```shell
helm install my-pbuf-registry ./charts/pbuf-registry \
  --set secrets.staticToken="your-admin-token"
```

---

## Common Deployment Scenarios

### 1. Development / Testing (Internal PostgreSQL + Web UI)

```shell
helm install my-pbuf-registry oci://ghcr.io/pbufio/charts/pbuf-registry \
  --set postgresql.enabled=true \
  --set postgresql.primary.persistence.enabled=true \
  --set secrets.staticToken="dev-admin-token" \
  --set ui.enabled=true \
  --set ui.publicEnabled=true
```

### 2. Production (External PostgreSQL + ACL / OIDC SSO + Web UI)

```shell
helm install my-pbuf-registry oci://ghcr.io/pbufio/charts/pbuf-registry \
  --set postgresql.enabled=false \
  --set secrets.databaseDSN="postgres://user:password@postgres.example.com:5432/pbuf_registry?sslmode=require" \
  --set secrets.staticToken="prod-admin-token" \
  --set service.grpc.auth.enabled=true \
  --set service.grpc.auth.type="oidc" \
  --set service.grpc.auth.oidc.issuerUrl="https://idp.example.com" \
  --set service.grpc.auth.oidc.clientId="pbuf-registry" \
  --set service.http.auth.enabled=true \
  --set service.http.auth.type="oidc" \
  --set service.http.auth.oidc.issuerUrl="https://idp.example.com" \
  --set service.http.auth.oidc.clientId="pbuf-registry" \
  --set ui.enabled=true \
  --set ui.oidc.enabled=true \
  --set ui.oidc.issuerUrl="https://idp.example.com" \
  --set ui.oidc.clientId="pbuf-ui"
```

---

## Values Reference

### Core Registry Server

| Key | Description | Default |
|---|---|---|
| `replicaCount` | Number of registry server replicas | `2` |
| `image.repository` | Registry server image repository | `ghcr.io/pbufio/registry` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag override (defaults to `.Chart.AppVersion`) | `""` |
| `imagePullSecrets` | Docker registry pull secret names | `[]` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full release resource name | `""` |
| `serviceAccount.create` | Create a Kubernetes ServiceAccount | `true` |
| `serviceAccount.automount` | Automount ServiceAccount API credentials | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.name` | Explicit ServiceAccount name | `""` |
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Additional pod labels | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| `resources` | CPU/Memory resource requests and limits for the server | `{}` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Node/pod affinities | `{}` |
| `customSidecarContainers` | Additional sidecar containers on the server pod | `[]` |

### Pod Disruption Budget

| Key | Description | Default |
|---|---|---|
| `podDisruptionBudget.enabled` | Create a `policy/v1` PodDisruptionBudget for registry server pods | `false` |
| `podDisruptionBudget.minAvailable` | Minimum available server pods, as an integer or percentage; mutually exclusive with `maxUnavailable` | `null` (effective `1` when both budgets are unset) |
| `podDisruptionBudget.maxUnavailable` | Maximum unavailable server pods, as an integer or percentage; mutually exclusive with `minAvailable` | `null` |

When enabled without an explicit budget, the PDB uses `minAvailable: 1`.
Both values default to `null` so that setting `maxUnavailable` alone replaces this
fallback without also having to clear `minAvailable`. If both values are non-null,
Helm rendering fails. Integer `0` is supported and is not treated as an unset value.

```shell
# Keep at least one registry server pod available during voluntary disruptions
helm install my-pbuf-registry ./charts/pbuf-registry \
  --set podDisruptionBudget.enabled=true

# Block voluntary evictions of registry server pods
helm install my-pbuf-registry ./charts/pbuf-registry \
  --set podDisruptionBudget.enabled=true \
  --set podDisruptionBudget.maxUnavailable=0
```

For percentage budgets, use values such as `minAvailable: "50%"` or
`maxUnavailable: "25%"`. Kubernetes rounds percentage budgets up; a percentage
`maxUnavailable` can therefore allow all pods of a single-replica deployment to be
evicted. Choose a budget appropriate for your replica count.

The PDB uses the registry Deployment's selector and does not cover background
workers, the Web UI, or PostgreSQL. It limits voluntary disruptions through the
Eviction API, such as `kubectl drain`; it does not prevent involuntary failures or
guarantee service availability. With `replicaCount: 1` and `minAvailable: 1`, the
only server pod cannot be voluntarily evicted, so a drain can remain blocked until
you increase the replica count or relax the budget. See the Kubernetes
[PDB documentation](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
for budget and eviction behavior.

### Service, Authentication & OIDC SSO

| Key | Description | Default |
|---|---|---|
| `service.type` | Kubernetes Service type | `ClusterIP` |
| `service.http.port` | HTTP REST server port | `8080` |
| `service.http.auth.enabled` | Enable HTTP authentication | `false` |
| `service.http.auth.type` | HTTP auth mode (`static-token`, `acl`, `oidc`) | `""` |
| `service.http.auth.oidc.issuerUrl` | OIDC Issuer URL for HTTP API | `""` |
| `service.http.auth.oidc.clientId` | OIDC Client ID for HTTP API | `""` |
| `service.http.auth.oidc.jwksUrl` | Optional explicit JWKS endpoint URL | `""` |
| `service.http.auth.oidc.usernameClaim` | JWT claim mapped to username | `"email"` |
| `service.http.auth.oidc.groupsClaim` | JWT claim mapped to user groups | `"groups"` |
| `service.http.auth.oidc.adminGroups` | Comma-separated OIDC groups granted global admin | `""` |
| `service.http.auth.oidc.autoProvision` | Auto-provision users on first OIDC login | `true` |
| `service.grpc.port` | gRPC server port | `6777` |
| `service.grpc.tls.enabled` | Enable TLS on gRPC server | `false` |
| `service.grpc.auth.enabled` | Enable gRPC authentication | `false` |
| `service.grpc.auth.type` | gRPC auth mode (`static-token`, `acl`, `oidc`) | `""` |
| `service.grpc.auth.oidc.issuerUrl` | OIDC Issuer URL for gRPC API | `""` |
| `service.grpc.auth.oidc.clientId` | OIDC Client ID for gRPC API | `""` |
| `service.grpc.auth.oidc.jwksUrl` | Optional explicit JWKS endpoint URL | `""` |
| `service.grpc.auth.oidc.usernameClaim` | JWT claim mapped to username | `"email"` |
| `service.grpc.auth.oidc.groupsClaim` | JWT claim mapped to user groups | `"groups"` |
| `service.grpc.auth.oidc.adminGroups` | Comma-separated OIDC groups granted global admin | `""` |
| `service.grpc.auth.oidc.autoProvision` | Auto-provision users on first OIDC login | `true` |
| `service.debug.port` | Debug & Prometheus metrics port | `8082` |

### Secrets & External Secrets Operator (ESO)

| Key | Description | Default |
|---|---|---|
| `secrets.create` | Create Kubernetes Secret resource | `true` |
| `secrets.databaseDSN` | External PostgreSQL connection string (when `postgresql.enabled=false`) | `""` |
| `secrets.staticToken` | Static admin token (`SERVER_STATIC_TOKEN`) | `""` |
| `secrets.uiApiToken` | Optional Web UI server-side proxy token (`UI_API_TOKEN`) | `""` |
| `secrets.driftWebhookUrl` | Optional Slack or HTTP webhook URL for drift notifications (`DAEMONS_PROTOPARSING_WEBHOOK_URL`) | `""` |
| `secrets.grpcTlsCert` | PEM-encoded gRPC TLS certificate | `""` |
| `secrets.grpcTlsKey` | PEM-encoded gRPC TLS private key | `""` |
| `secrets.eso.create` | Create ExternalSecret resource for External Secrets Operator | `false` |
| `secrets.eso.secretStoreRefName` | ESO SecretStore name | `""` |
| `secrets.eso.remoteRefKey` | Key in remote secret store | `""` |
| `secrets.eso.databaseDSNProperty` | Remote secret property for database DSN | `""` |
| `secrets.eso.serverStaticTokenProperty` | Remote secret property for static token | `""` |
| `secrets.eso.uiApiTokenProperty` | Remote secret property for Web UI proxy token | `""` |
| `secrets.eso.driftWebhookUrlProperty` | Remote secret property for drift webhook URL | `""` |
| `secrets.eso.serverGrpcTlsCertFileProperty` | Remote secret property for TLS cert | `""` |
| `secrets.eso.serverGrpcTlsKeyFileProperty` | Remote secret property for TLS key | `""` |

### Background Daemons

| Key | Description | Default |
|---|---|---|
| `background.compaction.enabled` | Deploy draft-tag compaction worker | `true` |
| `background.compaction.resources` | CPU/Memory resources for compaction worker | `{}` |
| `background.compaction.command` | Compaction command | `["/app/pbuf-registry"]` |
| `background.compaction.args` | Compaction arguments | `["compaction"]` |
| `background.protoparser.enabled` | Deploy proto-parsing & drift detection worker | `true` |
| `background.protoparser.resources` | CPU/Memory resources for proto-parser worker | `{}` |
| `background.protoparser.command` | Proto-parser command | `["/app/pbuf-registry"]` |
| `background.protoparser.args` | Proto-parser arguments | `["proto-parsing"]` |
| `background.protoparser.webhook.url` | Optional Slack or HTTP webhook URL for drift alerts (prefer `secrets.driftWebhookUrl`) | `""` |
| `background.protoparser.webhook.minSeverity` | Minimum drift severity to alert (`INFO`, `WARNING`, `CRITICAL`) | `"WARNING"` |
| `background.protoparser.webhook.format` | Webhook payload format (`""` auto-detects Slack vs JSON, or `"json"` / `"slack"`) | `""` |
| `background.protoparser.webhook.timeout` | HTTP request timeout for webhook delivery | `"10s"` |

### Web UI (`ui`)

| Key | Description | Default |
|---|---|---|
| `ui.enabled` | Deploy `pbuf-registry-ui` Deployment and Service | `false` |
| `ui.replicaCount` | Number of Web UI replicas | `1` |
| `ui.image.repository` | Web UI image repository | `ghcr.io/pbufio/pbuf-registry-ui` |
| `ui.image.pullPolicy` | Web UI image pull policy | `IfNotPresent` |
| `ui.image.tag` | Web UI image tag override (defaults to `.Chart.AppVersion`) | `""` |
| `ui.service.type` | Web UI Service type | `ClusterIP` |
| `ui.service.port` | Web UI Service port | `80` |
| `ui.publicEnabled` | Allow unauthenticated public read-only browsing | `false` |
| `ui.apiToken` | Optional server-side proxy token (`API_TOKEN`; prefer `secrets.uiApiToken`) | `""` |
| `ui.oidc.enabled` | Enable OIDC SSO login button in the Web UI | `false` |
| `ui.oidc.issuerUrl` | OIDC Issuer URL for browser PKCE flow | `""` |
| `ui.oidc.clientId` | OIDC Client ID for browser PKCE flow | `""` |
| `ui.oidc.redirectUri` | Optional explicit OIDC redirect URI | `""` |
| `ui.oidc.scopes` | OIDC scopes requested by the Web UI | `"openid profile email"` |
| `ui.resources` | CPU/Memory resources for Web UI | `{}` |

### Ingress

| Key | Description | Default |
|---|---|---|
| `ingress.enabled` | Enable Ingress resource | `false` |
| `ingress.target` | Target service when `ui.enabled=true` (`"ui"` or `"api"`) | `"ui"` |
| `ingress.className` | IngressClass name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress host rules | `[{"host": "chart-example.local", "paths": [{"path": "/", "pathType": "ImplementationSpecific"}]}]` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

### Internal PostgreSQL (`postgresql`)

| Key | Description | Default |
|---|---|---|
| `postgresql.enabled` | Deploy internal PostgreSQL StatefulSet (disable in production) | `true` |
| `postgresql.image.repository` | PostgreSQL image repository | `postgres` |
| `postgresql.image.tag` | PostgreSQL image tag | `18.1-alpine3.21` |
| `postgresql.image.pullPolicy` | PostgreSQL image pull policy | `IfNotPresent` |
| `postgresql.auth.username` | PostgreSQL username | `pbuf` |
| `postgresql.auth.password` | PostgreSQL password | `pbuf` |
| `postgresql.auth.database` | PostgreSQL database name | `pbuf_registry` |
| `postgresql.primary.persistence.enabled` | Enable PersistentVolumeClaim for PostgreSQL | `false` |
| `postgresql.primary.persistence.storageClass` | StorageClass name | `""` |
| `postgresql.primary.persistence.size` | PersistentVolumeClaim size | `8Gi` |
| `postgresql.primary.resources` | CPU/Memory resources for PostgreSQL | `{}` |

---

## Linting & E2E Testing

From the repository root:

```shell
# Lint the Helm chart
make lint-charts

# Run PDB rendering regression tests (requires Helm; no cluster needed)
go test ./charts -v

# Run Kind Kubernetes E2E tests
cd e2e/chart
./run-e2e-tests.sh
```

See [`e2e/chart/README.md`](../../e2e/chart/README.md) for Kind E2E test details.
