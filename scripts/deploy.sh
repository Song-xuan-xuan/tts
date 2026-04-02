#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

if [[ ! -f config/gateway.yaml ]]; then
  cp config/gateway.example.yaml config/gateway.yaml
fi

docker compose pull
docker compose up -d
