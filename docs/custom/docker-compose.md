# Docker Compose Support

Docker build and compose configuration for running CLIProxyAPI in containers.

## Usage

```bash
docker compose up -d
```

## Files

- `docker/Dockerfile` — Multi-stage Go build (includes web UI)
- `docker/entrypoint.sh` — Container entrypoint
- `Dockerfile` — Root Dockerfile with Go module proxy build args
- `docker-build-local.sh` — Build local Docker image and start services via Compose
- `docker-compose.yml` — Compose configuration (supports `CLI_PROXY_IMAGE` and `CLI_PROXY_PULL_POLICY` env vars)
- `.dockerignore` — Exclude patterns for build context
