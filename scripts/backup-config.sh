#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "${repo_root}"

mkdir -p backup
stamp="$(date +%Y%m%d-%H%M%S)"
tar -czf "backup/tts-stack-config-${stamp}.tar.gz" .env config/gateway.yaml
echo "backup/tts-stack-config-${stamp}.tar.gz"
