# TTS Stack

Self-contained deployment repository for an OpenAI-compatible TTS gateway in front of `mzzsfy/tts`.

## What this repository deploys

- `tts-gateway`, a Go HTTP gateway published to GHCR
- `mzzsfy/tts`, kept on the internal Docker network as the upstream speech engine
- Docker Compose packaging, deployment scripts, and config templates for running both services together

## Public endpoints

- `POST /v1/audio/speech`
- `GET /api/voices`
- `GET /healthz`

`POST /v1/audio/speech` requires `Authorization: Bearer <token>` and `Content-Type: application/json`. `GET /api/voices` returns a JSON object with top-level `default_voice` and `voices`.

## Quick start

```bash
cp .env.example .env
cp config/gateway.example.yaml config/gateway.yaml
docker compose up -d
```

After copying `config/gateway.example.yaml`, replace the placeholder token, set your real token value, and change that token to `enabled: true` before running `scripts/deploy.sh`. New deployments that still contain `sk_tts_prod_xxx` are rejected by `scripts/deploy.sh`.

If you want the guarded deploy flow instead of a direct compose start, run:

```bash
./scripts/deploy.sh
```

## NPM Setup

This repository does not deploy Nginx Proxy Manager. Configure your existing NPM instance to forward:

- domain: `tts.example.com`
- scheme: `http`
- forward host: your server IP or host reachable by NPM
- forward port: `18080` by default, or the value of `TTS_GATEWAY_PORT` in `.env`

Enable SSL and Force SSL in NPM.

## Example Speech Request

```bash
curl https://tts.example.com/v1/audio/speech \
  -H "Authorization: Bearer sk_tts_prod_real" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "input": "Hello from TTS Stack",
    "voice": "zh-CN-XiaoxiaoNeural"
  }' \
  --output reply.mp3
```

## Voice Catalog Request

```bash
curl https://tts.example.com/api/voices \
  -H "Authorization: Bearer sk_tts_prod_real"
```

Example response shape:

```json
{
  "default_voice": "zh-CN-XiaoxiaoNeural",
  "voices": [
    {
      "short_name": "zh-CN-XiaoxiaoNeural",
      "locale": "zh-CN",
      "gender": "Female"
    }
  ]
}
```

## Upgrade

```bash
docker compose pull
docker compose up -d
```

## Backup

```bash
./scripts/backup-config.sh
```
