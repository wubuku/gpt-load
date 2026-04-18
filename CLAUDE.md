# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GPT-Load is a high-performance AI API transparent proxy service that provides intelligent key management, load balancing, and failure recovery for OpenAI, Google Gemini, and Anthropic Claude APIs. It acts as a reverse proxy that manages multiple API keys across different providers.

## Build and Run Commands

```bash
# Build frontend and run server (production)
make run

# Run in development mode with race detection
make dev

# Execute key migration (encryption)
make migrate-keys ARGS="--from old-key --to new-key"

# Docker deployment
docker compose up -d
```

### Cross-compilation (for other platforms)

Dockerfile supports cross-compilation via `TARGETOS` and `TARGETARCH`:
```bash
# Build for macOS ARM64 via Docker
docker buildx build --platform darwin/arm64 -o type=local,dest=. .

# Build for macOS ARM64 locally (requires frontend build first)
cd web && npm install && npm run build && cd ..
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o gpt-load .
```

## Architecture Overview

### Dual-layer Configuration System

1. **Static (Environment Variables)**: Read at startup via `container.BuildContainer()`. Key vars: `AUTH_KEY`, `ENCRYPTION_KEY`, `DATABASE_DSN`, `REDIS_DSN`, `PORT`, `HOST`, `IS_SLAVE`

2. **Dynamic (Hot-Reload)**: Stored in database via `config.SystemSettingsManager`. System settings and group-specific overrides take effect without restart. Priority: Group Config > System Settings > Environment.

### Core Request Flow (`internal/proxy/server.go`)

```
HandleProxy → SelectSubGroup → GetChannel → SelectKey → executeRequestWithRetry
```

- `HandleProxy` (line 61): Entry point for all proxy requests at `/proxy/{group_name}/*`
- `executeRequestWithRetry` (line 116): Core retry logic with backoff
- Key selection uses atomic rotation via `keypool.KeyProvider.SelectKey`
- Stream vs normal responses handled separately via `GetStreamClient()`/`GetHTTPClient()`

### Channel Interface (`internal/channel/channel.go`)

All AI providers implement `ChannelProxy` interface:
- `BuildUpstreamURL`: Constructs upstream API URL
- `ModifyRequest`: Adds provider-specific auth headers
- `IsStreamRequest`: Detects streaming responses
- `ValidateKey`: Validates API key via test request

Factory at `channel/factory.go` creates appropriate channel based on group `ChannelType` (`openai`, `gemini`, `anthropic`, `openai-response`).

### Key Pool Architecture (`internal/keypool/provider.go`)

- Keys stored in Redis/Memory with HASH (`key:{id}`) and LIST (`group:{id}:active_keys`)
- `SelectKey` atomically rotates key from LIST and fetches details from HASH
- `UpdateStatus` handles success/failure asynchronously
- Failure count tracked per key; exceeds `blacklist_threshold` → key marked `invalid`

### Storage Abstraction (`internal/store/store.go`)

Store interface abstracts Redis vs in-memory:
- Redis: Supports pipelining, pub/sub for cache sync
- Memory: Fallback for single-instance deployment
- Key operations: `Rotate`, `LPush`, `LRem`, `HSet`, `HGetAll`

### Dependency Injection

Uses `go.uber.org/dig` in `internal/container/container.go`. All services declared in `AppParams` struct and constructor `NewApp` receives injected dependencies.

### Master-Slave Cluster

- Master (`IS_SLAVE=false`): Runs all services (cron checker, log cleanup, key validation)
- Slave (`IS_SLAVE=true`): Only handles proxy requests, subscribes to master cache updates
- Cache sync via Redis pub/sub (`internal/syncer/cache_syncer.go`)

### Group Types

- **Standard**: Single group with direct key pool
- **Aggregate**: Parent group containing weighted sub-groups; `SubGroupManager.SelectSubGroup` uses weighted random selection

### Error Handling

- `internal/errors/parser.go`: Parses upstream errors, categorizes as retryable
- `internal/errors/ignorable_errors.go`: Client-side errors (cancel, disconnect) that abort retries
- `internal/errors/uncounted_errors.go`: Errors that don't increment failure count

## Important Implementation Details

- API keys encrypted at rest via `ENCRYPTION_KEY`; decrypted on `SelectKey`
- Request logs stored encrypted with `KeyHash` for reverse lookup
- Model redirect rules in `Group.ModelRedirectMap` transform requests before sending
- Header rules applied via `utils.ApplyHeaderRules` using variable context
- Graceful shutdown reserves 5s for background services

## Frontend

Vue 3 + TypeScript SPA in `/web`. Built with Vite, uses Naive UI components. Embedded into Go binary via `//go:embed web/dist`. i18n supported (en-US, zh-CN, ja-JP).

## Database

GORM with auto-migrate. Models in `internal/models/types.go`. Migrations in `internal/db/migrations/`. Supports SQLite, MySQL, PostgreSQL.
