# Docker Compose Support

Docker build and compose configuration for running CLIProxyAPI in containers.

## Usage

```bash
docker compose up -d
```

## Files

- `docker/Dockerfile` — Multi-stage Go build
- `docker/entrypoint.sh` — Container entrypoint
- `docker-compose.yml` — Compose configuration
- `.dockerignore` — Exclude patterns for build context
