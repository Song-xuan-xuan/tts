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

awk '
  $1 == "tokens:" { in_tokens=1; next }
  in_tokens && /^[^[:space:]-]/ { in_tokens=0 }
  !in_tokens { next }
  $1 == "-" && $2 == "name:" {
    name=$3
    token=""
    enabled=""
    next
  }
  $1 == "token:" {
    token=$2
    next
  }
  $1 == "enabled:" {
    enabled=$2
    if (enabled != "true") {
      printf("refusing to deploy: token '%s' is disabled in config/gateway.yaml; enable it only after setting a real token\n", name) > "/dev/stderr"
      exit 1
    }
    if (token == "" || token == "sk_tts_prod_xxx") {
      printf("refusing to deploy: token '%s' still uses the example placeholder in config/gateway.yaml\n", name) > "/dev/stderr"
      exit 1
    }
  }
' "${config_path}"

docker compose pull
docker compose up -d
