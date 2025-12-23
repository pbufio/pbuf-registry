# pbuf-registry Examples

This directory contains example protobuf files and configurations to help you get started with pbuf-registry.

## What's included

- `hello-proto/` - A simple example module with:
  - `greeter.proto` - A basic gRPC service definition
  - `types.proto` - Common message types and enums
- `pbuf.yaml` - Configuration file for the example module

## Prerequisites

1. **pbuf-registry running**: Follow the [installation instructions](../README.md#installation) to start the registry
2. **pbuf installed**: Get it from [pbuf repository](https://github.com/pbufio/pbuf-cli)
3. **Environment configured**:
   ```shell
   export PBUF_REGISTRY_URL=https://localhost:6777
   export PBUF_REGISTRY_TOKEN=${SERVER_STATIC_TOKEN}
   ```

## Step-by-Step Tutorial

### Step 1: Register the module

First, register a new module in the registry (the module name comes from `pbuf.yaml`):

```shell
pbuf modules register
```

You should see a success message confirming the module was registered.

### Step 2: Push your first version

Navigate to the examples directory and push the module with a tag:

```shell
cd examples
pbuf modules push v1.0.0
```

This will:
- Read the `pbuf.yaml` configuration
- Collect all `.proto` files from `hello-proto/` directory
- Upload them to the registry with tag `v1.0.0`

### Step 3: Verify the module

Get information about your module, including all available tags:

```shell
pbuf modules get github.com/yourorg/hello-proto
```

You should see `v1.0.0` in the tags list.

### Step 4: Vendor the module (use it in another project)

Now, simulate using this module in another project:

```shell
# Create a new directory for testing
mkdir -p /tmp/test-project
cd /tmp/test-project

# Initialize a new pbuf project
cat > pbuf.yaml <<EOF
version: v1
module: github.com/yourorg/test-project
vendor:
  modules:
    - name: github.com/yourorg/hello-proto
      tag: v1.0.0
EOF

# Pull the dependencies
pbuf vendor
```

The proto files from `hello-proto` will be downloaded into your `vendor/` directory.

### Step 5: Push a new version

Let's make a change and push a new version. Go back to the examples directory:

```shell
cd /path/to/pbuf-registry/examples
```

Edit `hello-proto/greeter.proto` to add a new RPC method, then push:

```shell
pbuf modules push v1.1.0
```

Now you have two versions: `v1.0.0` and `v1.1.0` in the registry!

## Next Steps

- Explore the [REST API](../README.md#http) using the swagger documentation
- Check out the [gRPC API](../README.md#grpc) for programmatic access
- Try the Web UI at http://localhost:80 (if running via docker-compose)
- Learn about [advanced pbuf features](https://github.com/pbufio/pbuf-cli)

## Customization

To use these examples with your own module:

1. Change the `module` name in `pbuf.yaml`
2. Update the `go_package` option in the `.proto` files
3. Register your module: `pbuf modules register`
4. Push: `pbuf modules push v1.0.0`

## Troubleshooting

**Certificate errors?**
```shell
# If using self-signed certificates, skip verification (dev only!)
export PBUF_REGISTRY_INSECURE=true
```

**Connection refused?**
```shell
# Check if the registry is running
docker-compose ps

# Check the logs
docker-compose logs pbuf-registry
```

**Authentication errors?**
```shell
# Ensure your token is set correctly
echo $PBUF_REGISTRY_TOKEN

# If empty, export it:
export PBUF_REGISTRY_TOKEN=your-token-here
```
