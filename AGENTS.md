# AGENTS.md - AI Agent Guidelines for pbuf-registry

## Project Overview

pbuf-registry is a modern, open-source Protobuf Registry written in Go. It provides centralized storage and management for Protocol Buffer files, enabling teams to share and version .proto files across multiple repositories. The registry solves the common problem of manually copying protobuf files between projects by providing a central place to store, version, and sync protobuf definitions.

Key features include:
- Module registration and versioning with semantic tags
- Dependency management via pbuf.yaml configuration
- Role-based access control (ACL) with fine-grained permissions
- Background jobs for drift detection and proto parsing
- gRPC and HTTP APIs with TLS support
- Web UI for browsing modules and documentation

## Technology Stack

- **Language**: Go 1.25+
- **Framework**: Kratos (go-kratos) - microservice framework
- **Database**: PostgreSQL 18+ with migrations
- **API**: gRPC with HTTP transcoding, Protocol Buffers v3
- **Tools**: buf (proto linting/generation), mockery (test mocks)
- **Containerization**: Docker, Docker Compose
- **Testing**: Go standard testing with race detection

## Project Architecture

```
pbuf-registry/
├── cmd/                    # Application entrypoints
│   ├── main.go            # Main entry point
│   └── root.go            # CLI commands (server, compaction, proto-parsing)
├── internal/              # Private application code
│   ├── background/        # Background job workers
│   │   ├── driftdetection.go  # Detects breaking changes in protos
│   │   └── protoparsing.go    # Parses and indexes proto files
│   ├── config/            # Configuration loading
│   ├── data/              # Data layer (repositories, database)
│   ├── middleware/        # HTTP/gRPC middleware (auth, logging)
│   ├── model/             # Domain models and entities
│   ├── server/            # Server setup (gRPC, HTTP)
│   ├── mocks/             # Generated test mocks
│   └── utils/             # Utility functions
├── api/pbuf-registry/v1/  # Proto API definitions
├── gen/                   # Generated code (proto, certs)
├── migrations/            # SQL database migrations
├── third_party/           # Third-party proto dependencies
└── examples/              # Example usage and proto files
```

## Development Commands

```bash
# Environment setup
make init              # Install required Go tools (protoc plugins, mockery, linter)

# Building
make build             # Build main binary to bin/pbuf-registry
make build-migrations  # Build migrations binary to bin/pbuf-migrations
make build-docker      # Build Docker image

# Code generation
make api               # Generate Go code from API proto files
make vendor            # Pull proto dependencies via pbuf
make vendor-gen        # Generate Go code from vendored protos
make mocks             # Generate test mocks with mockery

# Quality
make test              # Run tests with coverage and race detection
make lint              # Run golangci-lint

# Running locally
make run               # Start dev environment (docker-compose.dev.yml)
make stop              # Stop dev environment
make run-prod          # Start production environment
make stop-prod         # Stop production environment
make cert-gen          # Generate TLS certificates
```

## Database Migrations

Migrations in `migrations/` use format `YYYYMMDDHHMMSS_description.sql` with goose up/down sections. Migrations run automatically on startup via pbuf-migrations.

## API Development

Proto files are in `api/pbuf-registry/v1/`. After modifying, run `make api` to regenerate Go code, then implement in `internal/server/`. Services: ModuleService, UserService, PermissionService.

## Configuration

Key environment variables (override config.yaml):
- `DATA_DATABASE_DSN` - PostgreSQL connection string
- `SERVER_GRPC_TLS_ENABLED/CERTFILE/KEYFILE` - TLS settings
- `SERVER_GRPC_AUTH_ENABLED/TYPE` - gRPC auth (acl, token)
- `SERVER_STATIC_TOKEN` - Admin token for full access

## Code Guidelines

1. Follow standard Go conventions and project patterns
2. Add tests for new functionality; use table-driven tests
3. Run `make lint` before committing
4. Proto changes require running `make api`
5. Database changes require new migration files
6. Use dependency injection via constructors
7. Handle errors explicitly; avoid panic in library code

## Access Control

Three permission levels:
- **Read**: Pull modules, list, view metadata
- **Write**: Read + push modules, register new modules
- **Admin**: Write + delete modules and tags

Permissions are per-module or global (using `*` as module name).

## Background Jobs

The registry runs background workers as separate services:
- **Compaction**: Cleans up old data
- **Proto Parsing**: Indexes proto file contents
- **Drift Detection**: Monitors for breaking changes

Each runs via CLI subcommands (e.g., `pbuf-registry compaction`).
