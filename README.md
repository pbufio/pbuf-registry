# Self-Hosted, Open-Source Protobuf Schema Registry (`pbuf-registry`)

[![Status](https://github.com/pbufio/pbuf-registry/actions/workflows/main.yml/badge.svg)](https://github.com/pbufio/pbuf-registry/actions/workflows/main.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/pbufio/pbuf-registry)](https://github.com/pbufio/pbuf-registry/releases)
[![Live Registry](https://img.shields.io/badge/live_registry-pbuf.io-10b981)](https://pbuf.io/modules)

`pbuf-registry` is an Apache-2.0, self-hosted Protocol Buffers schema registry written in Go—complete with **OIDC / OAuth2 SSO**, **per-module RBAC**, **multi-source vendoring** (`pbuf-registry`, `buf.build`, and Git), **automatic `option go_package` rewriting**, **breaking-change & transitive drift detection**, a **dark-mode Vue 3 Web UI**, and a **built-in Model Context Protocol (`pbuf mcp`) server** for AI coding agents.

<p align="center">
  <img src="docs/assets/cli-demo.svg" alt="pbuf CLI demo — multi-source vendoring with go_package patching and breaking change drift detection" width="900" />
</p>

---

## Table of Contents

- [Why `pbuf-registry`? (Comparison vs. Buf & Git)](#why-pbuf-registry)
- [The Ecosystem](#the-ecosystem)
- [Key Features](#key-features)
  - [Automatic `option go_package` Rewriting (`gen_out`)](#automatic-option-go_package-rewriting-gen_out)
  - [Hybrid Multi-Source Vendoring (`pbuf-registry`, `buf.build`, and Git)](#Multi-Source-Vendoring-pbuf-registry-bufbuild-and-Git)
  - [Schema Drift & Breaking Change Detection (`pbuf drift diff`)](#schema-drift--breaking-change-detection)
  - [Native MCP Server for AI Coding Agents (`pbuf mcp`)](#native-mcp-server-for-ai-coding-agents-pbuf-mcp)
  - [Per-Module RBAC & OIDC / OAuth2 SSO](#per-module-rbac--oidc--oauth2-sso)
  - [Web UI for Browsing & Incident Investigation](#web-ui-for-browsing--incident-investigation)
- [Quick Start (2 Minutes)](#quick-start-2-minutes)
- [CLI Installation (`pbuf`)](#cli-installation-pbuf)
- [Configuration (`pbuf.yaml`)](#configuration-pbufyaml)
  - [Vendoring from `pbuf-registry`](#1-vendoring-from-pbuf-registry)
  - [Vendoring from Buf Schema Registry (`buf.build`)](#2-vendoring-from-buf-schema-registry-bufbuild)
  - [Vendoring from Git Repositories](#3-vendoring-from-git-repositories)
  - [Automatic `go_package` Patching (`gen_out`)](#automatic-go_package-patching-gen_out)
- [CLI Reference (`pbuf`)](#cli-reference)
  - [Project & Authentication Commands](#project--authentication-commands)
  - [Module Commands (`pbuf modules`)](#module-commands-pbuf-modules)
  - [Metadata Commands (`pbuf metadata`)](#metadata-commands-pbuf-metadata)
  - [Drift Detection Commands (`pbuf drift`)](#drift-detection-commands-pbuf-drift)
  - [AI Coding Assistants (`pbuf mcp`)](#ai-coding-assistants-pbuf-mcp)
  - [User, Bot & Permission Commands (`pbuf users`)](#user-bot--permission-commands-pbuf-users)
- [Deploying `pbuf-registry`](#deploying-pbuf-registry)
  - [Docker Compose](#docker-compose)
  - [Helm (Kubernetes)](#helm-kubernetes)
  - [Web UI (`pbuf-registry-ui`)](#web-ui-pbuf-registry-ui)
- [Server CLI & Background Daemons](#server-cli--background-daemons)
- [Authentication & Access Control (RBAC & OIDC SSO)](#authentication--access-control-rbac--oidc-sso)
- [Server Configuration Reference](#server-configuration-reference)
- [CI/CD Integration](#cicd-integration)
- [API Reference](#api-reference)
- [Development & Contributing](#development--contributing)

---

## Why `pbuf-registry`?

Once gRPC microservices split across repositories, teams typically face an uncomfortable choice: copy `.proto` files via brittle Git scripts, or pay per-seat SaaS pricing for private schemas, SSO, and RBAC.

`pbuf-registry` solves three real pain points out of the box:
1. **Self-Hosted, Zero-Seat-Tax Private Registry**: Run a complete private registry (gRPC/REST server + background AST indexer + Vue 3 Web UI + PostgreSQL) in Docker Compose or Kubernetes under Apache-2.0—with **OIDC SSO and granular per-module RBAC included for free**.
2. **Automatic `option go_package` Rewriting (`gen_out`)**: Eliminate conflicting upstream `option go_package` declarations across Go repositories. Set `gen_out: gen` in `pbuf.yaml` and `pbuf vendor` automatically rewrites `option go_package` in downloaded `.proto` files to match your local `go.mod` module path.
3. **Gradual Migration via Multi-Source `pbuf.yaml`**: Pull internal schemas from `pbuf-registry`, upstream packages from `buf.build` (BSR), and raw `.proto` subtrees from Git repositories in a single manifest.

| Capability | `pbuf-registry` + `pbuf` | Buf Schema Registry (Free) | Buf BSR (Team / Enterprise) | Raw Git / `git submodule` / `protodep` |
| :--- | :---: | :---: | :---: | :---: |
| **License & Self-Hosting** | **Apache-2.0 (Self-Hosted or Cloud)** | SaaS only | Paid Enterprise self-hosted | DIY scripts |
| **Private Schemas without Per-Seat Pricing** | **Yes (Unlimited users & bots)** | Limited / public focus | Paid per-user seat tax | Yes |
| **OIDC / OAuth2 SSO & Per-Module RBAC** | **Included** | No | Paid / Enterprise tier | Relies on Git repo permissions |
| **Hybrid Vendoring (`pbuf` + `buf.build` + Git)** | **Yes (All 3 in one `pbuf.yaml`)** | BSR only | BSR only | Git only |
| **Automatic `option go_package` Patching (`gen_out`)** | **Built-in (`go.mod`-aware)** | Managed mode (compilation only) | Managed mode (compilation only) | Manual sed scripts |
| **Transitive Dependency Drift & Breaking Alerts** | **Built-in (`pbuf drift` + UI)** | Breaking lint only | Breaking lint + graph | None |
| **Native MCP Server (`pbuf mcp`) for AI Agents** | **Built-in (`stdio`)** | External | External | None |

---

## The Ecosystem

All components live in this monorepo and are released together under unified semantic versions:

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│  ┌──────────────┐    push / pull    ┌───────────────────────────┐    │
│  │     pbuf     │ ◄───────────────► │       pbuf-registry       │    │
│  │  (CLI tool)  │                   │   gRPC :6777 / HTTP :8080 │    │
│  └──────┬───────┘                   │   Debug/Metrics :8082     │    │
│         │ vendor                    │   PostgreSQL backend      │    │
│         ▼                           └─────────────▲─────────────┘    │
│  ┌──────────────┐                                 │                  │
│  │  buf.build   │                   ┌─────────────┴─────────────┐    │
│  │  & Git repos │                   │     pbuf-registry-ui      │    │
│  └──────────────┘                   │   Web UI (Vue 3 + nginx)  │    │
│                                     └───────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────┘
```

| Component | Description | Path | Artifact |
|---|---|---|---|
| **pbuf-registry** | Registry server & background daemons (gRPC, REST, RBAC, OIDC SSO, drift detection) | [`cmd/`](cmd/), [`internal/`](internal/) | `ghcr.io/pbufio/registry` |
| **pbuf (CLI)** | Command-line interface for initializing, pushing, vendoring (`pbuf-registry`, `buf.build`, Git), and administering the registry | [`cmd/pbuf/`](cmd/pbuf/), [`internal/cli/`](internal/cli/) | [GitHub Releases](https://github.com/pbufio/pbuf-registry/releases) |
| **pbuf-registry-ui** | Developer-first Vue 3 Web UI for browsing modules, proto files, parsed AST metadata, and drift alerts | [`ui/`](ui/) | `ghcr.io/pbufio/pbuf-registry-ui` |
| **Helm Chart** | Kubernetes Helm chart for deploying the server, background workers, PostgreSQL, and Web UI | [`charts/pbuf-registry/`](charts/pbuf-registry/) | `oci://ghcr.io/pbufio/charts/pbuf-registry` |

---

## Key Features

### Automatic `option go_package` Rewriting (`gen_out`)

Anyone managing Go gRPC microservices across multiple repositories has hit conflicting `option go_package` declarations in upstream `.proto` files. When you set `gen_out: gen` on any dependency in `pbuf.yaml`, `pbuf vendor` reads your local `go.mod` module path and automatically rewrites `option go_package` in every downloaded `.proto` file:

```diff
 syntax = "proto3";
 package acme.payment.v1;
-option go_package = "github.com/acme/payment-service/api/payment/v1;paymentv1";
+option go_package = "github.com/acme/order-service/gen/payment/v1";
```

### Multi-Source Vendoring (`pbuf-registry`, `buf.build`, and Git)

Consumer services can pull schemas from `pbuf-registry`, the Buf Schema Registry (`buf.build`), and Git repositories in a single `pbuf.yaml`:

```yaml
version: v1
name: myorg/order-service
registry:
  addr: registry.example.com:6777
export:
  paths:
    - api
modules:
  # 1. From pbuf-registry — pinned, auditable, drift-tracked, go_package patched
  - name: myorg/payment-service
    tag: v2.1.0
    out: third_party
    gen_out: gen

  # 2. From Buf Schema Registry (buf.build) — public or private (via BUF_TOKEN / ~/.netrc)
  - name: buf.build/bufbuild/protovalidate
    tag: v0.14.0
    out: third_party

  # 3. From a Git repository — pinned to a tag or branch
  - repository: https://github.com/googleapis/googleapis
    path: google/api
    tag: common-protos-1_3_2
    out: third_party
```

### Schema Drift & Breaking Change Detection

When a module version modifies, deletes, or incompatibly changes `.proto` files compared to prior tags or downstream dependencies, the background proto-parser indexes the AST and records drift events with severity levels (`INFO`, `WARNING`, `CRITICAL`) and upgrade recommendations. You can also run `pbuf drift diff` locally or in CI before pushing a tag:

```shell
# Compare local .proto edits against the latest published tag in the registry (exits non-zero on CRITICAL)
pbuf drift diff
pbuf drift diff --against v2.0.0 --format markdown

# List all unacknowledged drift events across the registry
pbuf drift list

# Drift events for a specific module, optionally filtered by tag
pbuf drift module payment-service --tag v2.1.0

# Evaluate transitive dependency drift status for a module
pbuf drift dependencies order-service --tag v1.4.0
```

### Native MCP Server for AI Coding Agents (`pbuf mcp`)

`pbuf` includes a built-in **Model Context Protocol (MCP)** server (`pbuf mcp`) over `stdio`. Connect **Claude Code**, **Cursor**, **Antigravity**, or **GitHub Copilot** directly to your organization's live Protobuf registry so AI coding assistants can inspect module tags, pull `.proto` definitions, query parsed AST metadata (messages, fields, RPCs, enums), and check breaking drift statuses without hallucinating wire types.

### Immutable Versioned Tags & Ephemeral Draft Tags

Every schema push gets a tag. Published tags are immutable: once pushed, the content at a tag never changes. Draft tags (`--draft`) can be overwritten during development and are automatically cleaned up after 7 days by the compaction daemon.

```shell
pbuf modules push v2.1.0               # Stable — permanent and immutable
pbuf modules push v2.1.0-rc1 --draft   # Draft — mutable, auto-expires after 7 days
```

### Per-Module RBAC & OIDC / OAuth2 SSO

Engineers and CI pipelines can authenticate using:
- **OIDC / OAuth2 SSO** (`pbuf login` via OAuth 2.0 Device Authorization Flow, or Web UI Authorization Code + PKCE Flow)
- **Scoped User & Bot Tokens** (`pbuf_user_*` / `pbuf_bot_*`) with explicit `read`, `write`, or `admin` permissions per module or globally (`*`)
- **Static Admin Token** (`SERVER_STATIC_TOKEN`) for bootstrap and administrative automation

### Web UI for Browsing & Incident Investigation

The dark-mode Vue 3 Web UI lets engineers browse registered modules, inspect `.proto` source files at any tag, explore parsed messages/services/enums/fields, and review dependency drift alerts — either in public read-only mode or authenticated via Bearer token or OIDC SSO.

<p align="center">
  <img src="docs/assets/ui-module-overview.svg" alt="pbuf-registry Web UI — Module Overview and AST Explorer" width="900" />
</p>

<p align="center">
  <img src="docs/assets/ui-drift-events.svg" alt="pbuf-registry Web UI — Transitive Dependency Drift and Breaking Change Alerts" width="900" />
</p>

---

## Quick Start (2 Minutes)

### Step 1 — Start the Registry Stack

```shell
git clone https://github.com/pbufio/pbuf-registry.git
cd pbuf-registry
make cert-gen                         # Generates TLS certificates in gen/certs/
export SERVER_STATIC_TOKEN=my-secret-token
make run-prod                         # Starts PostgreSQL, Registry, Daemons, and Web UI
```

- **gRPC API**: `localhost:6777`
- **REST API**: `http://localhost:8080`
- **Web UI**: `http://localhost:80`

### Step 2 — Install the `pbuf` CLI

```shell
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh
```

### Step 3 — Initialize, Authenticate & Push a Module

```shell
# Initialize pbuf.yaml for your module
pbuf init myorg/payment-service localhost:6777

# Save your registry token to ~/.netrc
pbuf auth "${SERVER_STATIC_TOKEN}"

# Register the module and push your .proto files as v1.0.0
pbuf modules register
pbuf modules push v1.0.0
```

### Step 4 — Vendor Dependencies

```shell
# Pull all configured dependencies into third_party/
pbuf vendor
```

See [`examples/`](examples/) for a complete end-to-end walkthrough with sample `.proto` files.

---

## CLI Installation (`pbuf`)

### Homebrew (macOS & Linux)

```shell
brew install pbufio/tap/pbuf
```

### Linux & macOS (Install Script)

Install the latest release to `/usr/local/bin`:

```shell
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh
```

Install a specific version or to a custom directory:

```shell
# Specific version
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh -s -- -v v0.10.0

# Custom install directory (no sudo required)
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh
```

### Using Go (`go install`)

Requires Go 1.25+:

```shell
go install github.com/pbufio/pbuf-registry/cmd/pbuf@latest
```

### Windows

**Option 1: PowerShell**

```powershell
$release = Invoke-RestMethod "https://api.github.com/repos/pbufio/pbuf-registry/releases/latest"
$version = $release.tag_name
$plainVersion = $version.TrimStart('v')
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$url = "https://github.com/pbufio/pbuf-registry/releases/download/$version/pbuf_${plainVersion}_windows_${arch}.zip"
$output = "$env:TEMP\pbuf.zip"

Invoke-WebRequest -Uri $url -OutFile $output
Expand-Archive -Path $output -DestinationPath "$env:TEMP\pbuf" -Force
Move-Item -Path "$env:TEMP\pbuf\pbuf.exe" -Destination "$env:USERPROFILE\bin\pbuf.exe" -Force
Remove-Item -Path $output, "$env:TEMP\pbuf" -Recurse -Force
```

**Option 2: Manual Binary Download**

Download pre-built archives for Linux, macOS, and Windows (`amd64` / `arm64`) from the [GitHub Releases page](https://github.com/pbufio/pbuf-registry/releases).

### Build from Source

```shell
make build-cli
# Binary is output to ./bin/pbuf
```

---

## Configuration (`pbuf.yaml`)

`pbuf` reads `pbuf.yaml` from the current working directory.

### Full `pbuf.yaml` Schema

```yaml
version: v1
name: myorg/my-service

registry:
  addr: registry.example.com:6777   # Registry gRPC address (host or host:port; defaults to :443 or :80)
  insecure: false                   # Set to true only for plaintext HTTP/gRPC in local dev
  oidc:                             # Optional defaults for `pbuf login`
    issuer: https://idp.example.com
    client_id: pbuf-cli

export:
  paths:                            # Directories whose .proto files are exported on `pbuf modules push`
    - api
    - proto

modules:
  # 1. pbuf-registry dependency
  - name: myorg/common-protos
    path: common/v1                 # Optional: restrict vendoring to a subfolder or single .proto file
    tag: v1.2.0
    out: third_party                # Destination directory for downloaded .proto files
    gen_out: gen                    # Optional: rewrites `option go_package` in downloaded .proto files

  # 2. Buf Schema Registry (buf.build) dependency
  - name: buf.build/bufbuild/protovalidate
    path: buf/validate              # Optional: filter by path prefix
    tag: v0.14.0                    # Tag, commit ref, or `branch:` (defaults to "main" if omitted)
    out: third_party
    gen_out: gen                    # Optional: rewrites `option go_package`

  # 3. Git repository dependency
  - repository: https://github.com/googleapis/googleapis
    path: google/api                # Required for Git modules: subfolder or single .proto file in repo
    tag: common-protos-1_3_2        # Git tag (or use `branch: main`)
    out: third_party
    gen_out: gen                    # Optional: rewrites `option go_package`
```

### Field Reference

| Field | Required | Description |
|---|---|---|
| `version` | Yes | Schema version (`v1`). |
| `name` | For `register` / `push` | Canonical module name in `pbuf-registry` (e.g., `myorg/payment-service`). |
| `registry.addr` | For registry commands | Hostname or `host:port` of the `pbuf-registry` gRPC server (e.g., `pbuf.cloud` or `localhost:6777`). |
| `registry.insecure` | No | When `true`, connects over plaintext gRPC without TLS (`false` by default). |
| `registry.oidc.issuer` | No | Default OIDC issuer URL used by `pbuf login` when `--issuer` and `PBUF_OIDC_ISSUER` are not set. |
| `registry.oidc.client_id` | No | Default OIDC client ID used by `pbuf login` when `--client-id` and `PBUF_OIDC_CLIENT_ID` are not set. |
| `export.paths` | For `push` | List of local directories to scan recursively for `.proto` files when pushing a module. |
| `modules` | For `vendor` | List of dependencies to vendor when running `pbuf vendor`. |

### 1. Vendoring from `pbuf-registry`

Modules with a `name` (not starting with `buf.build/`) and no `repository` field are pulled from the configured `registry.addr` over gRPC:

```yaml
modules:
  - name: pbufio/pbuf-registry
    tag: v0.9.0
    path: api/pbuf-registry/v1   # Optional: only vendor files under this prefix
    out: third_party
    gen_out: gen                 # Optional
```

### 2. Vendoring from Buf Schema Registry (`buf.build`)

Modules whose `name` starts with `buf.build/<owner>/<module>` are downloaded directly from the Buf Schema Registry reflection API (`https://buf.build/buf.reflect.v1beta1.FileDescriptorSetService/GetFileDescriptorSet`) and reconstructed into `.proto` files—without requiring a `pbuf-registry` server connection:

```yaml
modules:
  - name: buf.build/googleapis/googleapis
    tag: main
    path: google/type
    out: third_party
```

**Private BSR Modules Authentication:**
- Set the `BUF_TOKEN` environment variable (`export BUF_TOKEN=...`), **or**
- Add an entry for `buf.build` in `~/.netrc`:
  ```text
  machine buf.build
  password <your-buf-token>
  ```

### 3. Vendoring from Git Repositories

Modules with a `repository` URL are cloned with a shallow single-branch fetch (`depth: 1`) at the specified `tag` or `branch`:

```yaml
modules:
  - repository: https://github.com/protocolbuffers/protobuf
    path: examples/addressbook.proto
    tag: v24.4
    out: proto
```

**Private Git Repositories Authentication:**
Add credentials for your Git host in `~/.netrc`:

```text
machine github.com
login <github-username>
password <personal-access-token>
```

### Automatic `go_package` Patching (`gen_out`)

When a dependency in `modules` specifies `gen_out`, `pbuf vendor` inspects your local `go.mod` module name and automatically sets or rewrites `option go_package` in every vendored `.proto` file for that dependency to:

```text
<go.mod module name>/<gen_out>/<relative directory of .proto file>
```

For example, if your `go.mod` declares `module github.com/myorg/order-service`, `out: third_party`, and `gen_out: gen`, a vendored file at `third_party/payment/v1/payment.proto` will have its `go_package` patched to `github.com/myorg/order-service/gen/payment/v1`.

---

## CLI Reference

### Summary Table

| Command | Description |
|---|---|
| `pbuf init [name] [registry-url]` | Initialize `pbuf.yaml` interactively or via arguments |
| `pbuf auth <token>` | Save registry token to `~/.netrc` (`0600` permissions) |
| `pbuf login [--issuer <url>] [--client-id <id>] [--scope <scopes>]` | Authenticate via OAuth 2.0 / OIDC Device Authorization Flow |
| `pbuf vendor` | Vendor all dependencies (`pbuf-registry`, `buf.build`, Git) configured in `pbuf.yaml` |
| `pbuf modules register` | Register the module defined in `pbuf.yaml` |
| `pbuf modules push <tag> [--draft]` | Push `.proto` files from `export.paths` with a version tag |
| `pbuf modules list` | List all registered modules in the registry |
| `pbuf modules get [module_name]` | Get module details, tags, draft tags, packages, and dependencies |
| `pbuf modules update` | Update `pbuf-registry` dependencies in `pbuf.yaml` to their latest published tags |
| `pbuf modules delete-tag <tag>` | Delete a specific tag of the current module |
| `pbuf modules delete` | Delete the current module and all its tags |
| `pbuf metadata get [module_name] [tag]` | Retrieve parsed AST metadata (packages, messages, services, enums) for a module tag |
| `pbuf drift diff [module_name] [--against <tag>] [--format table\|markdown\|json] [--fail-on critical\|warning\|info\|none]` | Compare local `.proto` files against a registry tag and fail on breaking changes |
| `pbuf drift list [--unacknowledged-only]` | List schema drift events across all modules |
| `pbuf drift module <module_name> [--tag <tag>]` | List schema drift events for a specific module |
| `pbuf drift dependencies <module_name> [--tag <tag>]` | Check dependency drift status and upgrade recommendations for a module |
| `pbuf drift acknowledge <event_id> --by <user>` | Acknowledge a schema drift event by ID |
| `pbuf mcp` | Start a Model Context Protocol (MCP) server over `stdio` for AI coding agents |
| `pbuf users create <name> [--type user\|bot]` | Create a user or bot and output its one-time token |
| `pbuf users list [--page-size 50] [--page 0]` | List users and bots |
| `pbuf users get <id>` | Get user or bot details by ID |
| `pbuf users update <id> [--name <n>] [--active\|--inactive]` | Update a user or bot's name or active status |
| `pbuf users delete <id>` | Delete a user or bot and revoke all permissions |
| `pbuf users regenerate-token <id>` | Rotate a user or bot's authentication token |
| `pbuf users grant-permission <user_id> <module> --permission read\|write\|admin` | Grant module-scoped or global (`*`) permission |
| `pbuf users revoke-permission <user_id> <module>` | Revoke permission for a module or `*` |
| `pbuf users list-permissions <user_id>` | List all permissions granted to a user or bot |

---

### Project & Authentication Commands

#### `pbuf init`

Initializes a `pbuf.yaml` file in the current directory.
- Run without arguments to launch an interactive terminal prompt (asking for registry URL, module name, dependencies, and export paths).
- Run with positional arguments for non-interactive initialization:

```shell
# Interactive TUI prompt
pbuf init

# Non-interactive (defaults registry to pbuf.cloud when omitted)
pbuf init myorg/payment-service
pbuf init myorg/payment-service registry.example.com:6777
```

#### `pbuf auth`

Stores a static, user, or bot token in `~/.netrc` (creating the file with `0600` permissions if it does not exist) for the host configured in `pbuf.yaml` (`registry.addr`, or `localhost` if `pbuf.yaml` is not present):

```shell
pbuf auth pbuf_bot_abc123...
```

#### `pbuf login`

Authenticates with an OIDC identity provider using the **OAuth 2.0 Device Authorization Flow** (RFC 8628) and saves the resulting ID token / access token to `~/.netrc`.

```shell
# Using flags
pbuf login --issuer https://idp.example.com --client-id pbuf-cli --scope "openid profile email"

# Using environment variables
export PBUF_OIDC_ISSUER=https://idp.example.com
export PBUF_OIDC_CLIENT_ID=pbuf-cli
pbuf login

# Or using registry.oidc.issuer and registry.oidc.client_id from pbuf.yaml
pbuf login
```

#### `pbuf vendor`

Downloads all dependencies listed under `modules:` in `pbuf.yaml` (supporting `pbuf-registry`, `buf.build` BSR, and Git repositories) and applies `go_package` patching where `gen_out` is configured:

```shell
pbuf vendor
```

---

### Module Commands (`pbuf modules`)

All `pbuf modules` commands use the `name` and `registry` settings from `pbuf.yaml`.

```shell
# Register the module in pbuf-registry
pbuf modules register

# Push .proto files from export.paths with an immutable tag
pbuf modules push v1.0.0

# Push a temporary draft tag (mutable, auto-deleted after 7 days)
pbuf modules push v1.1.0-rc1 --draft

# List all modules in the registry
pbuf modules list

# Get module metadata, tags, and dependencies (uses name from pbuf.yaml if omitted)
pbuf modules get
pbuf modules get myorg/payment-service

# Update all pbuf-registry dependencies in pbuf.yaml to their latest published tag
pbuf modules update

# Delete a specific tag of the current module
pbuf modules delete-tag v1.1.0-rc1

# Delete the current module and all of its tags
pbuf modules delete
```

---

### Metadata Commands (`pbuf metadata`)

Retrieves parsed AST metadata (packages, messages, fields, nested types, enums, and gRPC services/methods) indexed by the background proto-parser:

```shell
# Uses module name from pbuf.yaml when omitted
pbuf metadata get
pbuf metadata get myorg/payment-service v1.0.0
```

---

### Drift Detection Commands (`pbuf drift`)

Inspects local uncommitted breaking changes, file-level drift events, and cross-module dependency drift:

```shell
# Compare local .proto files in export.paths against the latest (or specified) registry tag
pbuf drift diff
pbuf drift diff --against v1.2.0 --format markdown --fail-on critical

# List unacknowledged drift events (default --unacknowledged-only=true)
pbuf drift list

# List all drift events (including acknowledged)
pbuf drift list --unacknowledged-only=false

# Get drift events for a specific module (optionally filtered by tag)
pbuf drift module myorg/payment-service
pbuf drift module myorg/payment-service --tag v2.0.0

# Print a tabular report of dependency drift status and recommendations
pbuf drift dependencies myorg/order-service
pbuf drift dependencies myorg/order-service --tag v1.4.0

# Acknowledge a drift event by ID
pbuf drift acknowledge <event-id> --by alice
```

---

### AI Coding Assistants (`pbuf mcp`)

`pbuf mcp` runs a standard JSON-RPC 2.0 **Model Context Protocol (MCP)** server over `stdio`. It connects to the registry configured in `pbuf.yaml` (or via `PBUF_REGISTRY_ADDR` and `PBUF_REGISTRY_INSECURE` environment variables) using credentials from `~/.netrc`.

#### Configure in Claude Code / Cursor / Antigravity (`mcp.json`)

```json
{
  "mcpServers": {
    "pbuf-registry": {
      "command": "pbuf",
      "args": ["mcp"],
      "env": {
        "PBUF_REGISTRY_ADDR": "pbuf.cloud:6777"
      }
    }
  }
}
```

#### Available MCP Tools

| Tool Name | Description |
| :--- | :--- |
| `list_modules` | List registered protobuf modules with optional pagination (`page_size`, `page_token`). |
| `get_module` | Retrieve module details, published tags, draft tags, and packages (`name`). |
| `get_module_dependencies` | Get direct and transitive dependencies for a module tag (`name`, `tag`, `resolve_transitive`). |
| `get_module_metadata` | Retrieve parsed AST metadata (packages, messages, fields, enums, services, RPCs) for a module tag (`name`, `tag`). |
| `get_proto_files` | Pull raw `.proto` file contents for a module and tag (`name`, `tag`). |
| `list_drift_events` | List schema drift and breaking-change events across the registry (`unacknowledged_only`). |
| `get_module_drift_events` | List schema drift events for a specific module (`module_name`, `tag_name`). |
| `get_dependency_drift_status` | Evaluate dependency drift severities and upgrade recommendations for a module (`module_name`, `tag_name`). |

---

### User, Bot & Permission Commands (`pbuf users`)

Requires admin privileges (Static Admin Token, a user with `admin` permission on `*`, or an OIDC user in `adminGroups`).

```shell
# Create a human user or CI bot (outputs one-time token: pbuf_user_* or pbuf_bot_*)
pbuf users create alice --type user
pbuf users create ci-deployer --type bot

# List users with pagination
pbuf users list --page-size 50 --page 0

# Get a user by UUID
pbuf users get <user-id>

# Update user name or active status
pbuf users update <user-id> --name "Alice Smith"
pbuf users update <user-id> --inactive
pbuf users update <user-id> --active

# Rotate a user or bot token (immediately invalidates the old token)
pbuf users regenerate-token <user-id>

# Grant and revoke permissions (read, write, admin) for a module or globally ("*")
pbuf users grant-permission <user-id> myorg/payment-service --permission write
pbuf users grant-permission <user-id> "*" --permission read
pbuf users list-permissions <user-id>
pbuf users revoke-permission <user-id> myorg/payment-service

# Delete a user or bot
pbuf users delete <user-id>
```

---

## Deploying `pbuf-registry`

### Docker Compose

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)

```shell
git clone https://github.com/pbufio/pbuf-registry.git
cd pbuf-registry
make cert-gen
export SERVER_STATIC_TOKEN=your-strong-admin-token
make run-prod    # Starts PostgreSQL, Registry, Compaction, Proto-Parser, and Web UI
make stop-prod   # Stops the stack
```

Pin specific image versions via `REGISTRY_VERSION` and `REGISTRY_UI_VERSION` (both default to `latest`):

```shell
REGISTRY_VERSION=v0.9.0 REGISTRY_UI_VERSION=v0.9.0 SERVER_STATIC_TOKEN=your-token docker compose up -d
```

For local development with ACL seeding and auto-generated dev tokens:

```shell
make run         # Starts docker-compose.dev.yml and seeds sample ACL users/tokens
make stop        # Tears down dev environment
```

### Helm (Kubernetes)

The Helm chart is published to **GitHub Container Registry (OCI)** and the **GitHub Pages Helm Repository**:

```shell
# Option 1: Install directly from GHCR OCI registry
helm install my-pbuf-registry oci://ghcr.io/pbufio/charts/pbuf-registry \
  --version 0.9.0 \
  --set postgresql.enabled=false \
  --set secrets.databaseDSN="postgres://user:pass@host:5432/pbuf_registry?sslmode=require" \
  --set secrets.staticToken="your-admin-token" \
  --set ui.enabled=true

# Option 2: Install via Helm repository
helm repo add pbuf https://pbufio.github.io/pbuf-registry
helm repo update
helm install my-pbuf-registry pbuf/pbuf-registry \
  --set secrets.staticToken="your-admin-token"
```

See [`charts/pbuf-registry/README.md`](charts/pbuf-registry/README.md) for the full configuration values table, including External Secrets Operator (ESO), Ingress, Web UI, and OIDC SSO settings.

### Web UI (`pbuf-registry-ui`)

The Web UI is included in `docker-compose.yml` and the Helm chart (`ui.enabled=true`), or can be run standalone via Docker:

```shell
docker run -d \
  -p 8080:80 \
  -e API_BASE_URL=http://your-registry:8080 \
  -e API_TOKEN=${SERVER_STATIC_TOKEN} \
  -e PUBLIC_ENABLED=true \
  --name pbuf-ui \
  ghcr.io/pbufio/pbuf-registry-ui:latest
```

See [`ui/README.md`](ui/README.md) for full Web UI configuration, proxy architecture, and OIDC SSO setup.

---

## Server CLI & Background Daemons

The `pbuf-registry` server binary (`cmd/main.go`) provides three subcommands:

| Command | Description | Default Schedule |
|---|---|---|
| `pbuf-registry` | Runs the main registry server (gRPC `:6777`, HTTP `:8080`, Debug `:8082`) and applies SQL migrations on startup | Always running |
| `pbuf-registry compaction` | Runs the background compaction daemon that deletes expired draft tags (older than 7 days) and orphaned proto files | `0 * * * *` (hourly) |
| `pbuf-registry proto-parsing` | Runs the background proto-parsing and drift detection daemons that index `.proto` AST metadata and detect breaking changes | `*/1 * * * *` (every minute) |

---

## Authentication & Access Control (RBAC & OIDC SSO)

### Authentication Modes

Configure `SERVER_GRPC_AUTH_ENABLED=true` / `SERVER_HTTP_AUTH_ENABLED=true` and set `SERVER_GRPC_AUTH_TYPE` / `SERVER_HTTP_AUTH_TYPE` to one of:

1. **`static-token`** — Authenticates all requests against `SERVER_STATIC_TOKEN` (full admin access).
2. **`acl`** (or `token`) — Full role-based access control:
   - `SERVER_STATIC_TOKEN` grants full superadmin access.
   - Scoped `pbuf_user_*` and `pbuf_bot_*` tokens are looked up via SHA-256 prefix digest and verified against bcrypt hashes in PostgreSQL (`pgcrypto`).
3. **`oidc`** — Enables OIDC / OAuth2 JWT verification **plus** full fallback support for `SERVER_STATIC_TOKEN` and `pbuf_user_*` / `pbuf_bot_*` tokens:
   - Discovers the provider's `jwks_uri` via `/.well-known/openid-configuration` (or explicit `JWKSURL`) and verifies RS256/ES256 JWT signatures, `iss`, `aud` (`clientId`), and `exp`.
   - Automatically provisions a user record on first login when `autoProvision: true`.
   - Grants global `admin` permission automatically if the user's `groups` claim intersects `adminGroups`.

### Permission Levels

| Level | Capabilities |
|---|---|
| **Read** | List modules, pull `.proto` files, view module metadata, view drift events |
| **Write** | **Read** + register new modules, push module tags |
| **Admin** | **Write** + delete modules, delete tags, manage users and permissions (when scoped to `*`) |

Permissions can be granted for a specific module name (e.g., `myorg/payment-service`) or globally (`*`). Unauthenticated users are allowed read-only access to public read endpoints (`ListModules`, `GetModule`, `PullModule`, `GetModuleDependencies`, `GetMetadata`, `GetModuleDriftEvents`, `GetModuleDependencyDriftStatus`).

---

## Server Configuration Reference

Configuration is loaded from the embedded [`internal/config/config.yaml`](internal/config/config.yaml) and overridden by environment variables (replacing `.` with `_` in uppercase):

| Environment Variable | Config Key | Default | Description |
|---|---|---|---|
| `DATA_DATABASE_DSN` | `data.database.dsn` | `postgres://pbuf:pbuf@localhost:5432/pbuf_registry?sslmode=disable` | PostgreSQL connection string |
| `SERVER_STATIC_TOKEN` | — | (empty) | Static admin token for full registry access |
| `SERVER_HTTP_ADDR` | `server.http.addr` | `0.0.0.0:8080` | HTTP/REST server listen address |
| `SERVER_HTTP_TIMEOUT` | `server.http.timeout` | `30s` | HTTP request timeout |
| `SERVER_HTTP_AUTH_ENABLED` | `server.http.auth.enabled` | `false` | Enable authentication on the HTTP server |
| `SERVER_HTTP_AUTH_TYPE` | `server.http.auth.type` | `""` | Auth mode: `static-token`, `acl`, or `oidc` |
| `SERVER_HTTP_AUTH_OIDC_ISSUERURL` | `server.http.auth.oidc.issuerUrl` | `""` | OIDC Issuer URL |
| `SERVER_HTTP_AUTH_OIDC_CLIENTID` | `server.http.auth.oidc.clientId` | `""` | OIDC Client ID (audience) |
| `SERVER_HTTP_AUTH_OIDC_JWKSURL` | `server.http.auth.oidc.jwksUrl` | `""` | Optional explicit JWKS URL override |
| `SERVER_HTTP_AUTH_OIDC_USERNAMECLAIM` | `server.http.auth.oidc.usernameClaim` | `email` | JWT claim used as the username |
| `SERVER_HTTP_AUTH_OIDC_GROUPSCLAIM` | `server.http.auth.oidc.groupsClaim` | `groups` | JWT claim containing group memberships |
| `SERVER_HTTP_AUTH_OIDC_ADMINGROUPS` | `server.http.auth.oidc.adminGroups` | `[]` | Comma-separated or list of OIDC groups granted global admin |
| `SERVER_HTTP_AUTH_OIDC_AUTOPROVISION` | `server.http.auth.oidc.autoProvision` | `true` | Auto-create user records on first OIDC login |
| `SERVER_GRPC_ADDR` | `server.grpc.addr` | `0.0.0.0:6777` | gRPC server listen address |
| `SERVER_GRPC_TIMEOUT` | `server.grpc.timeout` | `30s` | gRPC request timeout |
| `SERVER_GRPC_TLS_ENABLED` | `server.grpc.tls.enabled` | `false` | Enable TLS on the gRPC server |
| `SERVER_GRPC_TLS_CERTFILE` | `server.grpc.tls.certFile` | `""` | Path to gRPC TLS server certificate |
| `SERVER_GRPC_TLS_KEYFILE` | `server.grpc.tls.keyFile` | `""` | Path to gRPC TLS private key |
| `SERVER_GRPC_AUTH_ENABLED` | `server.grpc.auth.enabled` | `false` | Enable authentication on the gRPC server |
| `SERVER_GRPC_AUTH_TYPE` | `server.grpc.auth.type` | `""` | Auth mode: `static-token`, `acl`, or `oidc` |
| `SERVER_GRPC_AUTH_OIDC_ISSUERURL` | `server.grpc.auth.oidc.issuerUrl` | `""` | OIDC Issuer URL for gRPC |
| `SERVER_GRPC_AUTH_OIDC_CLIENTID` | `server.grpc.auth.oidc.clientId` | `""` | OIDC Client ID for gRPC |
| `SERVER_GRPC_AUTH_OIDC_JWKSURL` | `server.grpc.auth.oidc.jwksUrl` | `""` | Optional explicit JWKS URL override for gRPC |
| `SERVER_GRPC_AUTH_OIDC_USERNAMECLAIM` | `server.grpc.auth.oidc.usernameClaim` | `email` | JWT claim used as the username |
| `SERVER_GRPC_AUTH_OIDC_GROUPSCLAIM` | `server.grpc.auth.oidc.groupsClaim` | `groups` | JWT claim containing group memberships |
| `SERVER_GRPC_AUTH_OIDC_ADMINGROUPS` | `server.grpc.auth.oidc.adminGroups` | `[]` | OIDC groups granted global admin |
| `SERVER_GRPC_AUTH_OIDC_AUTOPROVISION` | `server.grpc.auth.oidc.autoProvision` | `true` | Auto-create user records on first OIDC login |
| `SERVER_DEBUG_ADDR` | `server.debug.addr` | `0.0.0.0:8082` | Debug & Prometheus `/metrics` listen address |
| `DAEMONS_COMPACTION_CRON` | `daemons.compaction.cron` | `0 * * * *` | Cron schedule for draft tag compaction |
| `DAEMONS_PROTOPARSING_CRON` | `daemons.protoparsing.cron` | `*/1 * * * *` | Cron schedule for proto parsing & drift detection |

---

## CI/CD Integration

### GitHub Actions

```yaml
steps:
  - name: Checkout
    uses: actions/checkout@v4

  - name: Install pbuf CLI
    run: |
      curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh -s -- -v v0.9.0
      pbuf auth "${{ secrets.PBUF_TOKEN }}"

  - name: Vendor dependencies
    run: pbuf vendor

  - name: Push schema on release tag
    if: startsWith(github.ref, 'refs/tags/v')
    run: |
      pbuf modules register || true
      pbuf modules push "${{ github.ref_name }}"
```

### GitLab CI

```yaml
.pbuf_setup: &pbuf_setup
  before_script:
    - curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh -s -- -v v0.9.0
    - pbuf auth "${PBUF_TOKEN}"

vendor:
  <<: *pbuf_setup
  script:
    - pbuf vendor
```

---

## API Reference

- **gRPC (`:6777` by default)**: Primary API for all registry, metadata, drift, and user management operations. Proto definitions: [`api/pbuf-registry/v1/`](api/pbuf-registry/v1/)
- **REST (`:8080` by default)**: HTTP/JSON transcoded API. OpenAPI/Swagger specification: [`gen/pbuf-registry/v1/registry.swagger.json`](gen/pbuf-registry/v1/registry.swagger.json)
- **Debug & Metrics (`:8082` by default)**: Health check and Prometheus metrics (`/metrics`).

---

## Development & Contributing

Contributions are welcome! Please see [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.

### Common `make` Targets

```shell
# Setup & Code Generation
make init                 # Install required Go protoc plugins, mockery, and golangci-lint
make api                  # Regenerate Go/gRPC/HTTP/Swagger code from api/pbuf-registry/v1/
make mocks                # Regenerate test mocks with mockery

# Build
make build                # Build registry server binary to bin/pbuf-registry
make build-cli            # Build pbuf CLI binary to bin/pbuf
make build-migrations     # Build database migrations binary to bin/pbuf-migrations
make build-ui             # Build Web UI production bundle in ui/dist

# Test & Lint
make test                 # Run all Go unit tests (server + CLI) with race detector
make test-ui              # Run Web UI unit tests (node --test)
make test-ui-e2e          # Run Web UI Playwright E2E tests
make lint                 # Run golangci-lint
make lint-charts          # Lint Kubernetes Helm chart in charts/pbuf-registry/
make test-e2e             # Run binary + PostgreSQL E2E test suite
make test-compose-e2e     # Run full Docker Compose + CLI + Playwright UI E2E suite
```
