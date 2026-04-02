#!/usr/bin/env bash
set -euo pipefail

mkdir -p backup
stamp="$(date +%Y%m%d-%H%M%S)"
tar -czf "backup/tts-stack-config-${stamp}.tar.gz" .env config/gateway.yaml
echo "backup/tts-stack-config-${stamp}.tar.gz"
