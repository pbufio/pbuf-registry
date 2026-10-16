#!/usr/bin/env bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    log_error "helm is not installed."
    log_info "Install it from: https://helm.sh/docs/intro/install/"
    exit 1
fi

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${PROJECT_ROOT}"

log_info "Running helm lint..."
helm lint charts/pbuf-registry/

if command -v ct &> /dev/null; then
    log_info "Running chart-testing lint..."
    ct lint --target-branch main --chart-dirs charts
fi

log_info "${GREEN}All lint checks passed!${NC}"
