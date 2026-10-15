# E2E Tests for pbuf-registry Helm Chart

This directory contains end-to-end tests for the pbuf-registry Helm chart using Kind (Kubernetes in Docker).

## Prerequisites

To run the e2e tests locally, you need the following tools installed:

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) (Kubernetes in Docker)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) v3+

## Running Tests Locally

### Basic Usage

Run the e2e tests with default settings:

```bash
cd tests/e2e
./run-e2e-tests.sh
```

The script will:
1. Create a Kind cluster named `pbuf-registry-e2e`
2. Install the Helm chart with internal PostgreSQL
3. Wait for all components to be ready
4. Run health checks and connectivity tests
5. Test external database configuration
6. Clean up resources

### Configuration Options

You can customize the test execution using environment variables:

```bash
# Use a custom cluster name
CLUSTER_NAME=my-test-cluster ./run-e2e-tests.sh

# Use a custom namespace
NAMESPACE=test-ns ./run-e2e-tests.sh

# Use a custom release name
RELEASE_NAME=my-release ./run-e2e-tests.sh

# Set custom timeout (in seconds)
TIMEOUT=600 ./run-e2e-tests.sh

# Skip cleanup after tests (useful for debugging)
SKIP_CLEANUP=true ./run-e2e-tests.sh

# Keep the Kind cluster after tests (only delete Helm releases)
SKIP_CLUSTER_DELETE=true ./run-e2e-tests.sh
```

### Debugging Failed Tests

If tests fail and you want to inspect the cluster:

```bash
# Run tests without cleanup
SKIP_CLEANUP=true ./run-e2e-tests.sh

# Then inspect the cluster
kubectl get pods --all-namespaces
kubectl logs <pod-name>
kubectl describe pod <pod-name>

# When done, manually clean up
kind delete cluster --name pbuf-registry-e2e
```

## Running Tests in CI/CD

The e2e tests are automatically run in GitHub Actions on:
- Pull requests to `main` branch
- Pushes to `main` branch
- Manual workflow dispatch

See `.github/workflows/e2e.yml` for the workflow configuration.

### Manual Trigger

You can manually trigger the e2e tests from the GitHub Actions UI:
1. Go to the "Actions" tab in your repository
2. Select "E2E Tests" workflow
3. Click "Run workflow"

## Test Coverage

The e2e test suite covers:

### 1. Internal PostgreSQL Deployment
- PostgreSQL StatefulSet deployment
- Database readiness and health checks
- Proper storage configuration
- Security contexts and best practices

### 2. Application Deployment
- Main pbuf-registry service deployment
- Background jobs (compaction, protoparser)
- Health check endpoints
- Database connectivity

### 3. External Database Configuration
- Deploying with external database DSN
- Verifying connection to existing PostgreSQL instance
- Testing production-like setup

### 4. pbuf CLI Functional Testing
- Automatic installation of pbuf CLI
- Module registration via `pbuf modules register`
- Module publishing via `pbuf modules push`
- Module retrieval via `pbuf modules get`
- Module vendoring in consumer projects via `pbuf vendor`
- Verification of vendored proto files
- End-to-end workflow validation

## Architecture

### Kind Cluster Configuration

The Kind cluster is configured in `kind-config.yaml`:
- Single control-plane node
- Port mappings for HTTP (80), HTTPS (443), and gRPC (6777)
- Suitable for both local and CI/CD environments

### PostgreSQL Configuration

Two PostgreSQL configurations are available:

1. **Internal PostgreSQL** (for testing):
   - Deployed as part of the Helm chart
   - Uses StatefulSet with optional persistence
   - Configured in Helm values under `postgresql.enabled: true`

2. **External PostgreSQL** (for production):
   - User provides database DSN via `secrets.databaseDSN`
   - PostgreSQL deployment is disabled with `postgresql.enabled: false`
   - Mimics production setup with external managed database

## Best Practices

The PostgreSQL deployment follows best practices:

1. **Security**:
   - Non-root user (UID 999)
   - Security contexts configured
   - Secrets for credentials

2. **Reliability**:
   - Health probes (liveness and readiness)
   - Proper timeouts and thresholds
   - StatefulSet for stable network identity

3. **Production Ready**:
   - Persistent volume support
   - Configurable storage class
   - Resource limits and requests

4. **Conditional Deployment**:
   - Enable for development/testing
   - Disable for production (use external DB)

## Files

- `kind-config.yaml` - Kind cluster configuration
- `run-e2e-tests.sh` - Main test script with pbuf CLI functional tests
- `postgres.yaml` - Standalone PostgreSQL deployment for testing
- `example-module/` - Example protobuf module for functional testing
  - `pbuf.yaml` - Module configuration
  - `hello-proto/` - Example proto files (greeter.proto, types.proto)
- `README.md` - This file

## Troubleshooting

### Tests hang during PostgreSQL startup

Increase the timeout:
```bash
TIMEOUT=600 ./run-e2e-tests.sh
```

### Kind cluster creation fails

Clean up existing clusters:
```bash
kind delete cluster --name pbuf-registry-e2e
```

### Permission denied on script

Make sure the script is executable:
```bash
chmod +x run-e2e-tests.sh
```

### Tests fail in CI but pass locally

Check the GitHub Actions logs for specific error messages. Common issues:
- Resource constraints (memory/CPU)
- Network timeouts
- Docker rate limiting

## Contributing

When adding new tests:
1. Update the test script with new test scenarios
2. Ensure tests are idempotent
3. Add cleanup logic for new resources
4. Update this README with new coverage areas
