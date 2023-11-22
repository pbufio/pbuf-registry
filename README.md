# Free-Forever Open-Source Protobuf Registry

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

## Usage

### CLI

We recommend to use the [CLI](https://github.com/pbufio/pbuf-cli) to interact with the registry.

### API

#### HTTP

The registry provides a REST API (`:8080` port by default). You can find the swagger documentation [here](https://github.com/pbufio/pbuf-registry/blob/main/gen/v1/registry.swagger.json).

#### gRPC

The registry provides a gRPC API (`:6777` port by default). You can find the protobuf definition [here](https://github.com/pbufio/pbuf-registry/blob/main/api/v1/registry.proto)

## Development and Contributing

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