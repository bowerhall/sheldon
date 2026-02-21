#!/bin/bash
# Build the coder-sandbox image
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

docker build -t sheldon-coder-sandbox:latest .

echo "Built: sheldon-coder-sandbox:latest"
echo "This image will be used automatically when CODER_ISOLATED=true"
