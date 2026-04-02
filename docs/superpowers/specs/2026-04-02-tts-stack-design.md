# TTS Stack Design

**Date:** 2026-04-02

**Goal:** Build a GitHub-hosted, self-contained Docker deployment package that exposes an OpenAI-compatible TTS API through a Go gateway in front of `mzzsfy/tts`, with manual multi-token configuration, per-token defaults, and migration-friendly deployment.

## Scope

This design covers the first production-shaped version of the deployment package and API gateway. It includes:

- A standalone GitHub repository for deployment and release automation
- A Go-based `tts-gateway` published to GHCR
- A Docker Compose deployment package that runs `tts-gateway` and `mzzsfy/tts`
- OpenAI-compatible `POST /v1/audio/speech`
- `GET /api/voices` for token-filtered voice discovery
- `GET /healthz` for health checks
- Manual multi-token configuration in a YAML file
- Per-token defaults for voice and upstream execution behavior
- Hot reload for token and business policy configuration

This first version intentionally excludes:

- A management UI or admin API
- Request caching
- Rate limiting
- Saved audio files or temporary audio URLs
- Streaming TTS generation
- Caller-controlled `thread`, `shardLength`, `speed`, or `response_format`
- Bundling NPM into the deployment package

## Architecture

The system is split into three layers:

1. Existing Nginx Proxy Manager instance
2. `tts-gateway`
3. `mzzsfy/tts`

NPM remains outside this repository and continues to provide domain binding, TLS, and reverse proxying. The new repository ships a Docker Compose package that only starts the gateway and upstream TTS service.

Traffic flow:

```text
client / LLM / script
-> https://tts.example.com
-> NPM
-> tts-gateway
-> mzzsfy/tts
```

`tts-gateway` is the only public service. `mzzsfy/tts` stays on the internal Docker network and is never exposed directly to the public internet.

## Public API

### POST /v1/audio/speech

This endpoint remains OpenAI-compatible in request shape.

Required headers:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`

Accepted request body:

```json
{
  "model": "tts-1",
  "input": "Hello world",
  "voice": "zh-CN-XiaoxiaoNeural"
}
```

Rules:

- `input` is required
- `model` must equal `tts-1`
- `voice` is optional
- When `voice` is omitted, the gateway uses the token's default voice
- When `voice` is provided, it must be present in that token's `allowed_voices`
- Callers cannot set `thread`
- Callers cannot set `shardLength`
- Callers cannot set `speed`
- Callers cannot set `response_format`
- Output is always `mp3`

Successful responses return raw `audio/mpeg` bytes.

### GET /api/voices

Returns the current token's allowed voice list and effective default voice. The endpoint exists so clients do not need to hardcode or mirror the full upstream voice catalog.

Example response:

```json
{
  "default_voice": "zh-CN-XiaoxiaoNeural",
  "voices": [
    {
      "short_name": "zh-CN-XiaoxiaoNeural",
      "locale": "zh-CN",
      "gender": "Female"
    },
    {
      "short_name": "zh-CN-YunxiNeural",
      "locale": "zh-CN",
      "gender": "Male"
    }
  ]
}
```

### GET /healthz

Returns a simple health status for container checks and reverse proxy health monitoring. It does not expose sensitive upstream details.

## Configuration Model

The gateway loads a YAML configuration file from disk. This file is mounted into the container from the deployment package.

First version layout:

```yaml
server:
  port: 8080

upstream:
  base_url: http://tts:8080
  timeout_seconds: 90

defaults:
  thread: 1
  shard_length: 400
  max_text_length: 8000

tokens:
  - name: llm-prod
    token: sk_tts_prod_xxx
    enabled: true
    defaults:
      voice: zh-CN-XiaoxiaoNeural
      thread: 1
      shard_length: 400
      max_text_length: 8000
    allowed_voices:
      - zh-CN-XiaoxiaoNeural
      - zh-CN-YunxiNeural
```

### Configuration Semantics

- `server.port` is process-level and is not hot reloaded
- `upstream.base_url` is process-level and is not hot reloaded
- `upstream.timeout_seconds` is process-level and is not hot reloaded
- `defaults.*` provides system fallback values
- Each token may override fallback values through `tokens[].defaults`
- `allowed_voices` is mandatory for voice control
- `enabled: false` disables a token immediately after reload

## Hot Reload

The gateway supports hot reload only for business policy configuration. This includes:

- token add/remove
- token enable/disable
- token default voice changes
- token default thread changes
- token default shard length changes
- token text length limit changes
- token voice whitelist changes

The gateway does not hot reload:

- listen port
- upstream base URL
- process timeout and network bootstrap settings

Reload behavior:

- File watcher notices `gateway.yaml` changes
- Gateway validates the new business configuration
- If validation passes, the in-memory config swaps atomically
- Existing requests continue using the previous config snapshot
- New requests use the new config
- If validation fails, the gateway keeps the previous config and logs the reload failure

## Request Flow

For `POST /v1/audio/speech`, the gateway handles requests in this order:

1. Parse bearer token from `Authorization`
2. Find the matching token entry in the in-memory config
3. Reject when missing, disabled, or unknown
4. Validate `model`
5. Validate `input`
6. Enforce token-specific maximum text length
7. Resolve the effective voice
8. Validate the requested voice against the token whitelist
9. Resolve the effective `thread` and `shard_length` from token defaults
10. Call the upstream `mzzsfy/tts` service
11. Stream upstream audio bytes directly back to the client

The gateway is intentionally synchronous and stateless in the first version. It does not store generated audio and does not cache repeated requests.

## Upstream Mapping

The gateway remains OpenAI-compatible externally, but internally it maps requests onto the upstream `mzzsfy/tts` interface.

Mapping behavior:

- external `input` -> upstream `text`
- external `voice` -> upstream `voiceName`
- output format is fixed to upstream MP3 format
- token-configured `thread` -> upstream `thread`
- token-configured `shard_length` -> upstream `shardLength`

The gateway owns all internal policy translation. External clients do not learn or depend on upstream-specific tuning parameters.

## Error Model

The gateway normalizes errors to a small set of HTTP behaviors.

### 401 Unauthorized

Used for:

- missing bearer token
- malformed bearer token
- unknown token
- disabled token

### 400 Bad Request

Used for:

- missing `input`
- empty `input`
- unsupported `model`
- text length exceeded
- requested voice not allowed for that token

### 502 Bad Gateway

Used for:

- upstream request failures
- upstream invalid response
- upstream unexpected content type
- upstream internal errors

### 504 Gateway Timeout

Used for:

- upstream timeout

Error body format:

```json
{
  "error": {
    "type": "bad_gateway",
    "message": "upstream tts service failed"
  }
}
```

## Logging and Observability

The gateway logs at least the following fields per request:

- timestamp
- token name
- request text length
- effective voice
- effective thread
- effective shard length
- upstream duration
- final status code
- error summary, when applicable

The gateway must not log:

- raw bearer token
- full input text

`mzzsfy/tts` keeps its own monitor token and monitor endpoints. Those remain separate from gateway business authentication.

## Risk Controls

The upstream service has already shown instability under some long-text and concurrent scenarios. The first version therefore applies conservative defaults:

- default `thread` is `1`
- caller cannot override upstream concurrency parameters
- each token has a max text length
- the raw upstream service is never exposed publicly

These constraints intentionally bias toward reliability over flexibility.

## Repository Layout

The deployment package repository is designed for migration and release, not only for source development.

Proposed repository structure:

```text
tts-stack/
  compose.yaml
  .env.example
  README.md
  config/
    gateway.example.yaml
  scripts/
    deploy.sh
    backup-config.sh
  .github/
    workflows/
      ghcr-release.yml
```

### File Responsibilities

- `compose.yaml`
  - starts `tts-gateway` and `mzzsfy/tts`
- `.env.example`
  - contains deploy-time environment examples such as image tag and exposed host port
- `config/gateway.example.yaml`
  - provides the configuration template copied to `config/gateway.yaml`
- `README.md`
  - documents deployment, NPM setup, upgrade, migration, and rollback
- `scripts/deploy.sh`
  - wraps the common deploy flow
- `scripts/backup-config.sh`
  - saves a copy of `.env` and `config/gateway.yaml`
- `.github/workflows/ghcr-release.yml`
  - builds and pushes `tts-gateway` images to GHCR

## Deployment Package

The deployment package is intentionally self-contained except for the external NPM instance.

The package should be usable in either of these ways:

- clone the repository
- download a GitHub Release archive

Standard deployment flow on a new host:

1. Install Docker and Docker Compose
2. Clone the repository or download the release archive
3. Copy `.env.example` to `.env`
4. Copy `config/gateway.example.yaml` to `config/gateway.yaml`
5. Fill in tokens and token defaults
6. Run `docker compose up -d`
7. Point NPM to the host port exposed by `tts-gateway`

This creates a reproducible migration path if the server is replaced or lost.

## Compose Topology

The first version Compose stack contains only:

- `tts-gateway`
- `tts`

`tts-gateway` uses a published GHCR image:

```text
ghcr.io/<owner>/tts-gateway:<version>
```

`tts` uses the upstream image:

```text
mzzsfy/tts:<tag>
```

`tts` is attached to the internal Docker network and is not published directly.

## NPM Integration

NPM continues to be managed separately from this repository.

Expected proxy behavior:

- domain: `tts.example.com`
- scheme: `http`
- target host: server IP or host reachable by NPM
- target port: the host port bound by `tts-gateway`

TLS remains entirely in NPM.

## Migration and Recovery

To move to a new server:

1. Provision Docker
2. Restore repository contents
3. Restore `.env`
4. Restore `config/gateway.yaml`
5. Start with `docker compose up -d`
6. Repoint NPM

Critical data to back up:

- `.env`
- `config/gateway.yaml`
- external documentation of NPM proxy host configuration

Because the service does not store generated audio or token state in a database, migration is configuration-driven rather than data-driven.

## Non-Goals for First Release

The first release explicitly does not include:

- token CRUD UI
- usage billing
- rate-limit enforcement
- request/result caching
- saved audio file lifecycle
- async job queue
- multi-provider TTS abstraction
- caller-controlled advanced synthesis settings

## Acceptance Criteria

The design is considered satisfied when:

- a new server can deploy from the GitHub package without local source builds
- the gateway image is pulled from GHCR
- external clients can call `POST /v1/audio/speech` with bearer auth
- per-token defaults and allowed voices are enforced
- business config changes hot reload without container restart
- audio is returned directly as MP3 stream
- `tts` is not publicly exposed
- `GET /api/voices` returns token-filtered voices
- `GET /healthz` returns a simple healthy response
