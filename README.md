# TTS Stack

Self-contained deployment repository for an OpenAI-compatible TTS gateway in front of `mzzsfy/tts`.

## What this repository deploys

- `tts-gateway`, a Go HTTP gateway published to GHCR
- `mzzsfy/tts`, kept on the internal Docker network as the upstream speech engine
- Docker Compose packaging, deployment scripts, and config templates for running both services together

The default published gateway image is `ghcr.io/song-xuan-xuan/tts-gateway:latest`.

## Public endpoints

- `POST /v1/audio/speech`
- `GET /api/voices`
- `GET /healthz`

`POST /v1/audio/speech` requires `Authorization: Bearer <token>` and `Content-Type: application/json`. `GET /api/voices` also requires `Authorization: Bearer <token>` and returns a JSON object with top-level `default_voice` and `voices`.

## Quick start

```bash
cp .env.example .env
cp config/gateway.example.yaml config/gateway.yaml
# edit config/gateway.yaml:
#   - replace sk_tts_prod_xxx with a real token
#   - change enabled: false to enabled: true
# optional: if NPM runs on another machine, change TTS_GATEWAY_BIND_IP in .env
./scripts/deploy.sh
```

After copying `config/gateway.example.yaml`, replace the placeholder token, set your real token value, and change that token to `enabled: true` before running `./scripts/deploy.sh`. New deployments that still contain `sk_tts_prod_xxx` are rejected by the deploy script.

If you prefer to inspect the generated Docker Compose model before starting the stack, run:

```bash
docker compose config
```

If `docker compose pull` fails with a GHCR permission error, open the GitHub package page for `tts-gateway` under the `Song-xuan-xuan/tts` repository and set the package visibility to `Public`.

By default, `tts-gateway` only binds to `127.0.0.1` for safer same-host deployments with NPM. If your reverse proxy runs on a different machine, set `TTS_GATEWAY_BIND_IP=0.0.0.0` or another reachable private address in `.env` before deployment.

## NPM Setup

This repository does not deploy Nginx Proxy Manager. Configure your existing NPM instance to forward:

- domain: `tts.example.com`
- scheme: `http`
- forward host: `127.0.0.1` when NPM runs on the same machine, otherwise a host/IP reachable by NPM
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
