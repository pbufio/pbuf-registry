# Modern Open-Source Protobuf Registry

![Status](https://github.com/pbufio/pbuf-registry/actions/workflows/main.yml/badge.svg)

The registry provides a central place to store and manage your protobuf files.

## Motivation

The protobuf files are usually stored in the same repository as the code that uses them. 
This approach works well for small projects, but it becomes a problem when you have multiple repositories that use the same protobuf files. 
In this case, you have to copy manually the files to each repository, which is not only tedious but also error-prone.

The registry solves this problem by providing a central place to store and manage your protobuf files and sync the files with your repositories.

## Installation

### Docker Compose

#### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/install/)

#### Steps

1. Clone the repository
2. Generate certificates with `make certs-gen` command (they will appear in `gen/certs` folder) or put your own certificates in the folder
3. Export `SERVER_STATIC_TOKEN` with static authorization token
4. Run `make run-prod` to start the registry
5. Run `make stop-prod` to stop the running registry

#### Configuration

The registry can be configured with environment variables that overrides values in the `config/config.yaml` file. For instance, to change the database DSN you can set `DATA_DATABASE_DSN` environment variable that is reflect to `data.database.dsn` yaml property.

### Helm Chart

#### Prerequisites

- [Helm](https://helm.sh/docs/intro/install/)
- Postgres database should be provisioned separately

#### Installation

Add repository to Helm:

```shell
helm repo add pbuf https://pbufio.github.io/helm-charts
```

To install the chart with the release name `my-pbuf-registry`:

```shell
helm install my-pbuf-registry pbuf/pbuf-registry --set secrets.databaseDSN=<databaseDSN>
```

More information about the chart can be found in [the chart repository](https://github.com/pbufio/helm-charts)

## Quick Start (2 minutes)

Once the registry is running, try these commands to see it in action:

### 1. Install pbuf CLI

Install the latest version with a single command:

```shell
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-cli/main/install.sh | sh

# Or using wget
wget -qO- https://raw.githubusercontent.com/pbufio/pbuf-cli/main/install.sh | sh
```

For Windows or other installation methods, see the [pbuf CLI documentation](https://github.com/pbufio/pbuf-cli).

### 2. Initialize Your Project

```shell
# Initialize a new module (interactive)
pbuf init

# Or non-interactively
pbuf init my-module https://localhost:6777

# Authenticate with the registry
pbuf auth ${SERVER_STATIC_TOKEN}
```

### 3. Push Your First Module

```shell
# Create a proto file (or use existing ones)
mkdir -p api/v1
cat > api/v1/service.proto << 'EOF'
syntax = "proto3";
package mycompany.mymodule.v1;

message HelloRequest {
  string name = 1;
}

message HelloResponse {
  string message = 1;
}
EOF

# Push your first tag
pbuf modules push v1.0.0
```

### 4. Vendor Dependencies

```shell
# In another project, add the module to pbuf.yaml's modules section
# Then pull dependencies
pbuf vendor
```

**New to pbuf?** Check out the [examples/](examples/) directory for a complete walkthrough with sample protobuf files.

## Web UI

The registry includes a modern web interface for browsing and managing your protobuf modules.

### Features

- 🎨 **Modern Dark Theme** - Developer-first aesthetic with high contrast
- 📦 **Module Browsing** - Browse and search all registered protobuf modules
- 🏷️ **Version Management** - View and manage module tags and versions
- 📄 **Proto File Viewer** - Inspect proto file contents and structure
- 🔍 **Metadata Explorer** - Browse parsed protobuf messages, services, and fields
- 🔗 **Dependency Tracking** - View module dependencies and their relationships
- 📱 **Fully Responsive** - Optimized for desktop, tablet, and mobile devices

### Deployment

The UI is available as a separate project at [pbuf-registry-ui](https://github.com/pbufio/pbuf-registry-ui).

#### Docker Deployment

```shell
docker run -d \
  -p 8080:80 \
  -e API_BASE_URL=https://localhost:6777 \
  -e API_TOKEN=${SERVER_STATIC_TOKEN} \
  -e PUBLIC_ENABLED=true \
  --name pbuf-ui \
  pbufio/pbuf-registry-ui:latest
```

The UI will be available at `http://localhost:8080`.

For more information and configuration options, see the [pbuf-registry-ui documentation](https://github.com/pbufio/pbuf-registry-ui).

## Access Control (ACL)

The registry supports role-based access control with fine-grained permissions per module.

### Authentication

The registry supports two types of authentication:

1. **Admin Token** - The `SERVER_STATIC_TOKEN` environment variable serves as the admin token with full access to all operations
2. **User/Bot Tokens** - Generated tokens for users and bots with configurable permissions

### Permission Levels

- **Read** - Pull modules, list modules, view metadata
- **Write** - Read + Push modules, register new modules  
- **Admin** - Write + Delete modules and tags

Permissions can be granted per module or globally using `*` as the module name.

### User and Bot Management

Only admins can manage users, bots, and permissions.

#### Create a User/Bot

```shell
# Using grpcurl (requires generated proto files)
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d '{"name": "ci-bot", "type": "USER_TYPE_BOT"}' \
  localhost:6777 pbufregistry.v1.UserService/CreateUser
```

Returns a response with the user details and a generated token (only shown once):
```json
{
  "user": {"id": "...", "name": "ci-bot", "type": "USER_TYPE_BOT", ...},
  "token": "pbuf_bot_<random>"
}
```

#### Grant Permissions

```shell
# Grant read permission on a specific module
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d '{"user_id": "<user-id>", "module_name": "my-module", "permission": "PERMISSION_READ"}' \
  localhost:6777 pbufregistry.v1.UserService/GrantPermission

# Grant write permission on all modules
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d '{"user_id": "<user-id>", "module_name": "*", "permission": "PERMISSION_WRITE"}' \
  localhost:6777 pbufregistry.v1.UserService/GrantPermission
```

#### List Users

```shell
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  localhost:6777 pbufregistry.v1.UserService/ListUsers
```

#### Revoke Permissions

```shell
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d '{"user_id": "<user-id>", "module_name": "my-module"}' \
  localhost:6777 pbufregistry.v1.UserService/RevokePermission
```

#### Regenerate Token

```shell
grpcurl -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -d '{"id": "<user-id>"}' \
  localhost:6777 pbufregistry.v1.UserService/RegenerateToken
```

### Using User/Bot Tokens

Once a user or bot has been created and granted permissions, they can authenticate using their token:

```shell
# Configure pbuf CLI with user/bot token
export PBUF_REGISTRY_TOKEN="pbuf_bot_<random>"

# Now the bot can perform operations based on its permissions
pbuf modules pull my-module v1.0.0  # Requires read permission
pbuf modules push v1.0.0            # Requires write permission
```

### Token Security

- Tokens are generated using cryptographically secure random bytes (32 bytes)
- Tokens are encrypted in the database using PostgreSQL pgcrypto extension (bcrypt)
- Token format: `pbuf_<type>_<base64_random>` (e.g., `pbuf_bot_abc123...`)
- Tokens are only displayed once during creation or regeneration

For more details, see [ACL.md](ACL.md).

## Usage

### CLI

We recommend using the [pbuf CLI](https://github.com/pbufio/pbuf-cli) to interact with the registry.

#### Installation

**Linux/macOS:**
```shell
curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-cli/main/install.sh | sh
```

**Windows:**
Download the latest release from [pbuf-cli releases](https://github.com/pbufio/pbuf-cli/releases) and add to your PATH.

**From Source:**
```shell
go install github.com/pbufio/pbuf-cli@latest
```

#### Key Commands

- `pbuf init` - Initialize a new module with pbuf.yaml
- `pbuf auth <token>` - Authenticate with the registry
- `pbuf modules register` - Register a new module
- `pbuf modules push <tag>` - Push a module version
- `pbuf modules list` - List all available modules
- `pbuf modules get <module>` - Get module information
- `pbuf vendor` - Vendor module dependencies

For complete documentation and examples, see the [pbuf CLI repository](https://github.com/pbufio/pbuf-cli).

### API

#### HTTP

The registry provides a REST API (`:8080` port by default). You can find the swagger documentation [here](https://github.com/pbufio/pbuf-registry/blob/main/gen/pbuf-registry/v1/registry.swagger.json).

#### gRPC

The registry provides a gRPC API (`:6777` port by default). You can find the protobuf definition [here](https://github.com/pbufio/pbuf-registry/blob/main/api/pbuf-registry/v1/registry.proto)

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