#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
cd "${repo_root}"

config_path="${repo_root}/config/gateway.yaml"

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

if [[ ! -f "${config_path}" ]]; then
  cp config/gateway.example.yaml "${config_path}"
fi

server_port="$(awk '
  $1 == "server:" { in_server=1; next }
  in_server && $1 == "port:" { print $2; exit }
  in_server && /^[^[:space:]]/ { in_server=0 }
' "${config_path}")"

if [[ "${server_port}" != "8080" ]]; then
  echo "refusing to deploy: config/gateway.yaml server.port must be 8080 to match compose.yaml, found '${server_port:-missing}'" >&2
  exit 1
fi

if grep -Fq "sk_tts_prod_xxx" "${config_path}"; then
  echo "refusing to deploy: config/gateway.yaml still contains the example placeholder token sk_tts_prod_xxx" >&2
  exit 1
fi

docker compose pull
docker compose up -d
