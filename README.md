# Modern Open-Source Protobuf Registry

[![Status](https://github.com/pbufio/pbuf-registry/actions/workflows/main.yml/badge.svg)](https://github.com/pbufio/pbuf-registry/actions/workflows/main.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/pbufio/pbuf-registry)](https://github.com/pbufio/pbuf-registry/releases)

The registry provides a central place to store and manage your protobuf files.

## Motivation

Protobuf schemas are shared contracts between services. In a monorepo, proximity solves this — every team sees the same file. Once services split across repositories, you're copying `.proto` files by hand, vendoring from arbitrary Git branches, and hoping all the copies stay in sync. They won't.

`pbuf-registry` is a self-hosted schema registry for Protocol Buffers. It gives every `.proto` module a canonical home with versioned releases, a full audit trail, drift detection, and per-module access control — so schema state is observable and schema changes are attributable.

## The Ecosystem

`pbuf-registry` is the server component of a three-part toolchain:

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  ┌──────────────┐    push/pull     ┌──────────────────────┐    │
│  │  pbuf-cli    │ ◄──────────────► │   pbuf-registry      │    │
│  │  (CLI tool)  │                  │   (this repo)        │    │
│  └──────────────┘                  │   gRPC :6777         │    │
│                                    │   REST :8080         │    │
│  ┌──────────────┐    browse/view   │   PostgreSQL backend │    │
│  │ pbuf-        │ ◄──────────────► └──────────────────────┘    │
│  │ registry-ui  │                                               │
│  │  (Web UI)    │                                               │
│  └──────────────┘                                               │
└─────────────────────────────────────────────────────────────────┘
```

| Component | Description | Repo |
|---|---|---|
| **pbuf-registry** | Registry server — stores modules, enforces RBAC, exposes gRPC + REST | [pbufio/pbuf-registry](https://github.com/pbufio/pbuf-registry) |
| **pbuf-cli** | CLI for pushing, pulling, vendoring, and managing modules and users | [pbufio/pbuf-cli](https://github.com/pbufio/pbuf-cli) |
| **pbuf-registry-ui** | Web UI for browsing modules, inspecting proto files, and exploring metadata | [pbufio/pbuf-registry-ui](https://github.com/pbufio/pbuf-registry-ui) |

---

## Key Features

### Schema Drift Detection

When a service vendor-locks a schema version but the deployed binary diverges from it — due to a bad rollout, a skipped `pbuf vendor`, or an in-place copy that got stale — `pbuf drift` surfaces it before it becomes an incident.

```shell
# List all unacknowledged drift events across every module
pbuf drift list

# Drift events for a specific module, optionally filtered by tag
pbuf drift module payment-service
pbuf drift module payment-service --tag v2.1.0
```

### Immutable Versioned Tags

Every schema push gets a semver tag. Tags are immutable: once pushed, the content at a tag never changes. Draft tags auto-expire after 7 days and are safe to use in feature branches and CI.

```shell
pbuf modules push v2.1.0               # Stable — permanent
pbuf modules push v2.1.0-rc1 --draft   # Draft — expires in 7 days
```

Consumers pin to an exact tag in `pbuf.yaml`. `pbuf modules update` bumps all dependencies to latest when you're ready.

### Per-Module Access Control

CI pipelines and individual engineers get separate tokens with explicit permissions per module. A bot that pushes `payment-service` schemas has no write access to `user-service`. Tokens are bcrypt-encrypted in Postgres via `pgcrypto` and shown exactly once.

```shell
pbuf users create ci-bot --type bot
# → Returns a one-time token: pbuf_bot_

pbuf users grant-permission  payment-service --permission write
pbuf users grant-permission  "*" --permission read
```

### Vendor From Registry and Git in the Same Config

Consumer services can pull schemas from the registry (versioned, audited) and from a public Git repository in a single `pbuf.yaml`:

```yaml
modules:
  # From the registry — pinned, auditable, rollback-safe
  - name: myorg/payment-service
    tag: v2.1.0
    out: third_party

  # From a public Git repo — open-source protos, pinned to a tag
  - repository: https://github.com/googleapis/googleapis
    path: google/api
    tag: common-protos-1_3_2
    out: third_party
```

### Web UI for Incident Investigation

During an incident, the registry UI lets any engineer look up exactly which proto files were in a given module tag, inspect field-level metadata, and trace dependency relationships — without needing CLI access or digging through Git history.

Deploy it as a read-only, unauthenticated interface for the whole org, or lock it down behind a token.

---

## Quick Start (2 Minutes)

### Step 1 — Start the Registry

```shell
git clone https://github.com/pbufio/pbuf-registry.git
cd pbuf-registry
make certs-gen                        # Generates TLS certs in gen/certs/
export SERVER_STATIC_TOKEN=
make run-prod                         # Starts registry + postgres via Docker Compose
```

### Step 2 — Install the CLI

```shell
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-cli/main/install.sh | sh

# From source (requires Go)
go install github.com/pbufio/pbuf-cli@latest
```

Windows: download a pre-built binary from [pbuf-cli releases](https://github.com/pbufio/pbuf-cli/releases).

### Step 3 — Push a Module

```shell
# Initialize pbuf.yaml for the service that owns the schema
pbuf init payment-service https://localhost:6777
pbuf auth ${SERVER_STATIC_TOKEN}

# Push the current .proto files as a versioned tag
pbuf modules push v1.0.0
```

### Step 4 — Vendor Into a Consumer Service

```shell
# In a consumer service repo — add the module to pbuf.yaml, then:
pbuf vendor
# Proto files appear in third_party/ at the pinned version
```

See [`examples/`](examples/) for a complete walkthrough with sample protobuf files.


---

## Installation

### Docker Compose

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)

```shell
git clone https://github.com/pbufio/pbuf-registry.git
cd pbuf-registry
make certs-gen
export SERVER_STATIC_TOKEN=
make run-prod    # start
make stop-prod   # stop
```

Configuration is driven by environment variables that map to `config/config.yaml` using dot-notation as underscores. For example, `data.database.dsn` becomes `DATA_DATABASE_DSN`:

```shell
DATA_DATABASE_DSN=postgres://user:pass@host/dbname
SERVER_STATIC_TOKEN=my-secret-token
```

### Helm (Kubernetes)

```shell
helm repo add pbuf https://pbufio.github.io/helm-charts

helm install my-pbuf-registry pbuf/pbuf-registry \
  --set secrets.databaseDSN=
```

Postgres must be provisioned separately. See the [Helm chart repository](https://github.com/pbufio/helm-charts) for all values.

### Web UI

```shell
docker run -d \
  -p 8080:80 \
  -e API_BASE_URL=https://your-registry:6777 \
  -e API_TOKEN=${SERVER_STATIC_TOKEN} \
  -e PUBLIC_ENABLED=true \
  --name pbuf-ui \
  pbufio/pbuf-registry-ui:latest
```

Available at `http://localhost:8080`. Set `PUBLIC_ENABLED=true` for unauthenticated read-only browsing. See [pbuf-registry-ui](https://github.com/pbufio/pbuf-registry-ui) for full configuration.

---

## CI/CD Integration

### GitHub Actions

```yaml
- name: Configure pbuf auth
  run: |
    echo "machine your-registry.internal" > ~/.netrc
    echo "token ${{ secrets.PBUF_TOKEN }}" >> ~/.netrc
    chmod 600 ~/.netrc

- name: Vendor protos
  run: pbuf vendor

- name: Push schema on release tag
  if: startsWith(github.ref, 'refs/tags/')
  run: pbuf modules push ${{ github.ref_name }}
```

### GitLab CI

```yaml
.pbuf_auth: &pbuf_auth
  before_script:
    - echo "machine your-registry.internal" > ~/.netrc
    - echo "token ${PBUF_TOKEN}" >> ~/.netrc
    - chmod 600 ~/.netrc

vendor:
  <<: *pbuf_auth
  script:
    - pbuf vendor
```

---

## Access Control (ACL)

### Authentication

Two authentication modes:

- **Admin Token** — `SERVER_STATIC_TOKEN` environment variable. Full access to all operations.
- **User / Bot Tokens** — Scoped tokens generated per engineer or CI bot, with configurable permissions per module.

### Permission Levels

| Level | Capabilities |
|---|---|
| **Read** | List modules, pull proto files, view metadata |
| **Write** | Read + push new module versions, register modules |
| **Admin** | Write + delete modules and tags |

Permissions can be scoped to a single module name or applied globally with `*`.

### Managing Users and Bots

```shell
# Create a CI bot (token shown once)
pbuf users create ci-bot --type bot

# Grant scoped permissions
pbuf users grant-permission  payment-service --permission write
pbuf users grant-permission  "*" --permission read

# Audit and manage
pbuf users list
pbuf users list-permissions 
pbuf users revoke-permission  payment-service
pbuf users regenerate-token    # Rotate a compromised token
```

### Token Security

Tokens are generated from 32 bytes of cryptographically secure random data, stored as bcrypt hashes via PostgreSQL's `pgcrypto` extension. Format: `pbuf_<type>_<base64>`. Tokens are displayed only once — at creation or rotation time.

For full ACL documentation see [ACL.md](ACL.md).

---

## CLI Reference

Full documentation: [pbufio/pbuf-cli](https://github.com/pbufio/pbuf-cli)

| Command | Description |
|---|---|
| `pbuf init [name] [registry-url]` | Initialize `pbuf.yaml` for a module |
| `pbuf auth <token>` | Save registry token to `~/.netrc` |
| `pbuf modules register` | Register a new module in the registry |
| `pbuf modules push <tag> [--draft]` | Push proto files with a version tag |
| `pbuf modules list` | List all modules |
| `pbuf modules get [module]` | Get module info and available tags |
| `pbuf modules update` | Update `pbuf.yaml` deps to latest tags |
| `pbuf modules delete-tag <tag>` | Delete a specific tag |
| `pbuf vendor` | Pull all dependencies into `third_party/` |
| `pbuf metadata get <module> <tag>` | Inspect parsed messages, services, and fields |
| `pbuf drift list [--unacknowledged-only]` | List schema drift events |
| `pbuf drift module <module> [--tag]` | Drift events for a specific module |
| `pbuf users create <n> --type user\|bot` | Create a user or bot |
| `pbuf users grant-permission <id> <module> --permission read\|write\|admin` | Grant access |

---

## API

### gRPC (`:6777` by default)

Primary API for all registry operations. Proto definition: [`api/pbuf-registry/v1/registry.proto`](api/pbuf-registry/v1/registry.proto)

### REST (`:8080` by default)

HTTP/JSON API for integrations. Swagger: [`gen/pbuf-registry/v1/registry.swagger.json`](gen/pbuf-registry/v1/registry.swagger.json)

---

## Development and Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:
- How to report issues and request features
- Pull request process
- Code style guidelines
- Issue templates and labels

### Prerequisites

- [Go](https://golang.org/doc/install)
- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Make](https://www.gnu.org/software/make/)

### Build

- Run `make build` to build the registry
- Run `make build-in-docker` to build linux binaries in docker

### Test

- Run `make test` to run the tests.

### Test the Registry

- Run `make run` to start the registry and test it.
- Run `make stop` to stop the running registry.
